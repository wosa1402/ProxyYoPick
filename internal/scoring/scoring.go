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
// IPQS results are cached for 30 days; Scamalytics/AbuseIPDB are cached daily.
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
	now := time.Now()
	today := now.Format("2006-01-02")

	// Build unique IP -> proxy index map
	ipIndex := make(map[string][]int)
	var uniqueIPs []string
	for i, p := range proxies {
		if _, exists := ipIndex[p.IP]; !exists {
			uniqueIPs = append(uniqueIPs, p.IP)
		}
		ipIndex[p.IP] = append(ipIndex[p.IP], i)
	}

	// Per-IP: determine which scorers need querying based on per-service cache expiry
	var toQuery []string
	queryPlans := make(map[string][]scorer)
	cachedScores := make(map[string]cachedScore)
	for _, ip := range uniqueIPs {
		entry, hasCache := cache.get(ip)

		var needed []scorer
		for _, s := range scorers {
			if !hasCache || !isFresh(entry, s.Name(), now) {
				needed = append(needed, s)
			}
		}

		if len(needed) == 0 {
			cachedScores[ip] = entry
		} else {
			toQuery = append(toQuery, ip)
			queryPlans[ip] = needed
			if hasCache {
				cachedScores[ip] = entry // preserve existing cached values
			}
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

		result := queryAllScorers(ctx, queryPlans[ip], ip)
		merged := mergeScores(cachedScores[ip], result, today)
		cache.set(ip, merged)
		cachedScores[ip] = merged

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

// isFresh checks if a specific scorer's cached value is still valid.
// IPQS: 30-day cache; Scamalytics/AbuseIPDB: daily cache.
func isFresh(entry cachedScore, scorerName string, now time.Time) bool {
	switch scorerName {
	case "ipqs":
		if entry.IPQS == nil {
			return false
		}
		dateStr := entry.IPQSDate
		if dateStr == "" {
			dateStr = entry.Date // legacy entries without IPQSDate
		}
		d, err := time.Parse("2006-01-02", dateStr)
		return err == nil && now.Sub(d) < 30*24*time.Hour
	case "scamalytics":
		return entry.Scamalytics != nil && entry.Date == now.Format("2006-01-02")
	case "abuseipdb":
		return entry.AbuseIPDB != nil && entry.Date == now.Format("2006-01-02")
	}
	return false
}

// mergeScores merges newly queried scores into an existing cache entry.
func mergeScores(old, new cachedScore, today string) cachedScore {
	result := old
	result.Date = today
	if new.IPQS != nil {
		result.IPQS = new.IPQS
		result.IPQSDate = today
	}
	if new.Scamalytics != nil {
		result.Scamalytics = new.Scamalytics
	}
	if new.AbuseIPDB != nil {
		result.AbuseIPDB = new.AbuseIPDB
	}
	return result
}
