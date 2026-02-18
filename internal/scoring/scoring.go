package scoring

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/efan/proxyyopick/internal/model"
)

// Config holds API keys and cache configuration for scoring.
type Config struct {
	IPQSKey         string
	ScamalyticsUser string
	ScamalyticsKey  string
	AbuseIPDBKey    string
	CachePath       string // empty = default path
}

// HasAnyKey returns true if at least one API key is configured.
func (c Config) HasAnyKey() bool {
	return c.IPQSKey != "" || (c.ScamalyticsUser != "" && c.ScamalyticsKey != "") || c.AbuseIPDBKey != ""
}

// scorer is the internal interface each API client implements.
type scorer interface {
	Name() string
	Score(ctx context.Context, ip string) (int, error)
}

// ScoreProxies enriches proxies with fraud/abuse scores from configured APIs.
// It uses a daily disk cache to avoid redundant queries.
// Proxies are mutated in-place (same pattern as geo.LookupCountries).
func ScoreProxies(ctx context.Context, proxies model.ProxyList, cfg Config) {
	if !cfg.HasAnyKey() {
		return
	}

	// Build list of enabled scorers
	var scorers []scorer
	if cfg.IPQSKey != "" {
		scorers = append(scorers, newIPQSClient(cfg.IPQSKey))
	}
	if cfg.ScamalyticsUser != "" && cfg.ScamalyticsKey != "" {
		scorers = append(scorers, newScamalyticsClient(cfg.ScamalyticsUser, cfg.ScamalyticsKey))
	}
	if cfg.AbuseIPDBKey != "" {
		scorers = append(scorers, newAbuseIPDBClient(cfg.AbuseIPDBKey))
	}

	// Load cache
	cache := loadCache(cfg.CachePath)
	today := time.Now().Format("2006-01-02")

	// Build unique IP -> proxy index map
	ipIndex := make(map[string][]int)
	var uniqueIPs []string
	for i, p := range proxies {
		if _, exists := ipIndex[p.IP]; !exists {
			uniqueIPs = append(uniqueIPs, p.IP)
		}
		ipIndex[p.IP] = append(ipIndex[p.IP], i)
	}

	// Separate cached (fresh) from uncached IPs
	var toQuery []string
	cachedScores := make(map[string]cachedScore)
	for _, ip := range uniqueIPs {
		if entry, ok := cache.get(ip); ok && entry.Date == today {
			cachedScores[ip] = entry
		} else {
			toQuery = append(toQuery, ip)
		}
	}

	total := len(uniqueIPs)
	cached := total - len(toQuery)
	fmt.Printf("🔍 IP 评分: %d 个唯一 IP，%d 已缓存，%d 需查询\n", total, cached, len(toQuery))

	// Query uncached IPs sequentially with per-IP parallel API calls
	for i, ip := range toQuery {
		select {
		case <-ctx.Done():
			fmt.Println()
			goto done
		default:
		}

		result := queryAllScorers(ctx, scorers, ip)
		result.Date = today
		cache.set(ip, result)
		cachedScores[ip] = result

		fmt.Printf("\r🔍 评分进度: %d/%d", cached+i+1, total)

		// Rate limit delay between IPs (skip after last)
		if i < len(toQuery)-1 {
			select {
			case <-ctx.Done():
				fmt.Println()
				goto done
			case <-time.After(200 * time.Millisecond):
			}
		}
	}
	if len(toQuery) > 0 {
		fmt.Println()
	}

done:
	// Save cache to disk
	if err := cache.save(); err != nil {
		slog.Warn("failed to save score cache", "error", err)
	}

	// Apply scores to proxies
	for ip, entry := range cachedScores {
		scores := model.IPScores{
			IPQS:        entry.IPQS,
			Scamalytics: entry.Scamalytics,
			AbuseIPDB:   entry.AbuseIPDB,
		}
		for _, idx := range ipIndex[ip] {
			proxies[idx].Scores = scores
		}
	}
}

// queryAllScorers queries all enabled scorers for a single IP in parallel.
func queryAllScorers(ctx context.Context, scorers []scorer, ip string) cachedScore {
	var mu sync.Mutex
	var wg sync.WaitGroup
	result := cachedScore{}

	for _, s := range scorers {
		wg.Add(1)
		go func(s scorer) {
			defer wg.Done()
			score, err := s.Score(ctx, ip)
			if err != nil {
				slog.Debug("scoring failed", "api", s.Name(), "ip", ip, "error", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			v := score
			switch s.Name() {
			case "ipqs":
				result.IPQS = &v
			case "scamalytics":
				result.Scamalytics = &v
			case "abuseipdb":
				result.AbuseIPDB = &v
			}
		}(s)
	}

	wg.Wait()
	return result
}
