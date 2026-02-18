package geo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/efan/proxyyopick/internal/model"
)

const (
	batchURL  = "http://ip-api.com/batch"
	batchSize = 100
	rateDelay = 1500 * time.Millisecond // ip-api.com free tier: ~45 req/min
	fields    = "query,status,country,countryCode,proxy,hosting,mobile,isp"
)

// ipAPIResponse is the response from ip-api.com batch endpoint.
type ipAPIResponse struct {
	Query       string `json:"query"`
	Status      string `json:"status"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	Proxy       bool   `json:"proxy"`
	Hosting     bool   `json:"hosting"`
	Mobile      bool   `json:"mobile"`
	ISP         string `json:"isp"`
}

// batchRequest is a single item in the batch request body.
type batchRequest struct {
	Query  string `json:"query"`
	Fields string `json:"fields"`
}

// classifyQuality returns a quality label based on IP characteristics.
// Priority: proxy (worst) < datacenter < mobile < residential (best)
func classifyQuality(r ipAPIResponse) string {
	if r.Proxy {
		return "proxy"
	}
	if r.Hosting {
		return "datacenter"
	}
	if r.Mobile {
		return "mobile"
	}
	return "residential"
}

// LookupCountries fills geo and quality fields for each proxy.
// It uses ip-api.com batch API (100 IPs per request, free, no key needed).
func LookupCountries(ctx context.Context, proxies model.ProxyList) {
	if len(proxies) == 0 {
		return
	}

	total := len(proxies)
	fmt.Printf("🌍 正在查询 %d 个 IP 的归属地和质量...\n", total)

	// Build IP -> proxy index map (one IP may appear in multiple proxies)
	ipIndex := make(map[string][]int, total)
	var uniqueIPs []string
	for i, p := range proxies {
		if _, exists := ipIndex[p.IP]; !exists {
			uniqueIPs = append(uniqueIPs, p.IP)
		}
		ipIndex[p.IP] = append(ipIndex[p.IP], i)
	}

	// Process in batches
	done := 0
	for start := 0; start < len(uniqueIPs); start += batchSize {
		end := start + batchSize
		if end > len(uniqueIPs) {
			end = len(uniqueIPs)
		}
		batch := uniqueIPs[start:end]

		results, err := queryBatch(ctx, batch)
		if err != nil {
			slog.Warn("geo lookup batch failed", "error", err)
			continue
		}

		for _, r := range results {
			if r.Status != "success" {
				continue
			}
			for _, idx := range ipIndex[r.Query] {
				proxies[idx].Country = r.Country
				proxies[idx].CountryCode = r.CountryCode
				proxies[idx].IsProxy = r.Proxy
				proxies[idx].IsHosting = r.Hosting
				proxies[idx].IsMobile = r.Mobile
				proxies[idx].ISP = r.ISP
				proxies[idx].Quality = classifyQuality(r)
			}
		}

		done += len(batch)
		fmt.Printf("\r🌍 查询进度: %d/%d", done, len(uniqueIPs))

		// Rate limiting: pause between batches (skip after last batch)
		if end < len(uniqueIPs) {
			select {
			case <-ctx.Done():
				fmt.Println()
				return
			case <-time.After(rateDelay):
			}
		}
	}
	fmt.Println()
}

func queryBatch(ctx context.Context, ips []string) ([]ipAPIResponse, error) {
	reqBody := make([]batchRequest, len(ips))
	for i, ip := range ips {
		reqBody[i] = batchRequest{
			Query:  ip,
			Fields: fields,
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, batchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var results []ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return results, nil
}
