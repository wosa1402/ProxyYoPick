package appleban

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/efan/proxyyopick/internal/model"
	"golang.org/x/net/proxy"
)

const (
	appleURL = "https://iforgot.apple.com/password/verify/appleid?language=en_US"
	workers  = 20
)

// CheckAppleBan tests whether each proxy's IP is banned by Apple.
// Proxies are mutated in-place (same pattern as geo.LookupCountries / scoring.ScoreProxies).
// Only pass successful (reachable) proxies — failed proxies can't make outbound requests.
func CheckAppleBan(ctx context.Context, proxies model.ProxyList, timeout time.Duration) {
	if len(proxies) == 0 {
		return
	}

	// Deduplicate by IP — Apple bans by IP, not port
	ipIndex := make(map[string][]int)
	var uniqueIPs []string
	for i, p := range proxies {
		if _, exists := ipIndex[p.IP]; !exists {
			uniqueIPs = append(uniqueIPs, p.IP)
		}
		ipIndex[p.IP] = append(ipIndex[p.IP], i)
	}

	total := len(uniqueIPs)
	fmt.Printf("🍎 Apple 封禁检测: %d 个唯一 IP，并发 %d\n", total, workers)

	// Build jobs: for each unique IP, pick the first proxy with that IP
	type job struct {
		ip    string
		proxy model.Proxy
	}
	jobs := make(chan job, total)
	go func() {
		for _, ip := range uniqueIPs {
			idx := ipIndex[ip][0]
			jobs <- job{ip: ip, proxy: proxies[idx]}
		}
		close(jobs)
	}()

	// Results
	type result struct {
		ip     string
		banned bool
	}
	results := make(chan result, total)

	var done atomic.Int64

	// Worker pool
	var wg sync.WaitGroup
	w := workers
	if w > total {
		w = total
	}
	for i := 0; i < w; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				banned := checkOne(ctx, j.proxy, timeout)
				results <- result{ip: j.ip, banned: banned}

				n := done.Add(1)
				fmt.Printf("\r🍎 Apple 检测进度: %d/%d", n, total)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	banMap := make(map[string]bool, total)
	for r := range results {
		banMap[r.ip] = r.banned
	}
	fmt.Println()

	// Apply results to all proxies sharing each IP
	for ip, banned := range banMap {
		b := banned
		for _, idx := range ipIndex[ip] {
			proxies[idx].AppleBanned = &b
		}
	}

	bannedCount := 0
	for _, b := range banMap {
		if b {
			bannedCount++
		}
	}
	slog.Info("apple ban check completed", "total", total, "banned", bannedCount, "ok", total-bannedCount)
}

// checkOne checks if a single proxy's IP is banned by Apple.
// Returns true if banned, false otherwise.
func checkOne(ctx context.Context, p model.Proxy, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer, err := proxy.SOCKS5("tcp", p.Address(), nil, proxy.Direct)
	if err != nil {
		slog.Debug("apple check: socks5 dial failed", "proxy", p.Address(), "error", err)
		return false // connection error ≠ Apple ban
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, appleURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("apple check: request failed", "proxy", p.Address(), "error", err)
		return false // timeout/connection error ≠ Apple ban
	}
	defer resp.Body.Close()

	// Layer 1: status code check
	if resp.StatusCode == 403 || resp.StatusCode == 503 {
		return true
	}

	// Layer 2: body content check (for 200 responses that mask a 403)
	if resp.StatusCode == 200 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024)) // read up to 64KB
		if err != nil {
			return false
		}
		content := string(body)
		if strings.Contains(content, "<center><h1>") && strings.Contains(content, "403 Forbidden") {
			return true
		}
	}

	return false
}
