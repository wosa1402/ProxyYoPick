package store

import (
	"sync"
	"time"

	"github.com/efan/proxyyopick/internal/model"
)

// PoolName identifies a proxy pool.
type PoolName string

const (
	PoolAuto   PoolName = "auto"
	PoolManual PoolName = "manual"
)

// ProxyHealth tracks the health state of a single proxy across test cycles.
type ProxyHealth struct {
	ConsecFails    int       // consecutive failure count (10 = dead)
	Dead           bool      // true = moved to failed pool
	FirstSeen      time.Time // when this proxy was first discovered
	LastSeen       time.Time // last time it appeared in a scrape
	LastTested     time.Time // last time it was tested
	RetestDate     string    // "2006-01-02": which day the last dead-retest was on
	RetestFails    int       // failures during today's dead-retest (3 = skip rest of day)
}

// pool holds results and state for a single proxy pool.
type pool struct {
	results   []model.TestResult
	updatedAt time.Time
	running   bool
	proxies   model.ProxyList            // accumulated proxy list
	health    map[string]*ProxyHealth    // key = ip:port
}

// Store holds test results for multiple pools with thread-safe access.
type Store struct {
	mu    sync.RWMutex
	pools map[PoolName]*pool
}

func New() *Store {
	return &Store{
		pools: map[PoolName]*pool{
			PoolAuto:   {},
			PoolManual: {},
		},
	}
}

func (s *Store) SetResults(name PoolName, results []model.TestResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)
	p.results = results
	p.updatedAt = time.Now()
}

func (s *Store) GetResults(name PoolName) ([]model.TestResult, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.getPool(name)
	out := make([]model.TestResult, len(p.results))
	copy(out, p.results)
	return out, p.updatedAt
}

func (s *Store) SetRunning(name PoolName, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getPool(name).running = running
}

func (s *Store) IsRunning(name PoolName) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getPool(name).running
}

func (s *Store) getPool(name PoolName) *pool {
	p, ok := s.pools[name]
	if !ok {
		p = &pool{}
		s.pools[name] = p
	}
	return p
}

// Stats returns summary statistics for a pool.
type Stats struct {
	Total       int    `json:"total"`
	Success     int    `json:"success"`
	Fail        int    `json:"fail"`
	AvgMs       int64  `json:"avg_ms"`
	FastestMs   int64  `json:"fastest_ms"`
	UpdatedAt   string `json:"updated_at"`
	Running     bool   `json:"running"`
	Pool        string `json:"pool"`
	Accumulated int    `json:"accumulated"` // total accumulated proxies
	Live        int    `json:"live"`        // non-dead proxies
	DeadCount   int    `json:"dead_count"`  // dead proxies
}

func (s *Store) GetStats(name PoolName) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p := s.getPool(name)
	stats := Stats{
		Total:   len(p.results),
		Running: p.running,
		Pool:    string(name),
	}

	if p.updatedAt.IsZero() {
		stats.UpdatedAt = ""
	} else {
		stats.UpdatedAt = p.updatedAt.Format("2006-01-02 15:04:05")
	}

	var totalMs int64
	stats.FastestMs = -1
	for _, r := range p.results {
		if r.Success {
			stats.Success++
			totalMs += r.LatencyMs
			if stats.FastestMs < 0 || r.LatencyMs < stats.FastestMs {
				stats.FastestMs = r.LatencyMs
			}
		} else {
			stats.Fail++
		}
	}

	if stats.Success > 0 {
		stats.AvgMs = totalMs / int64(stats.Success)
	}
	if stats.FastestMs < 0 {
		stats.FastestMs = 0
	}

	// Health stats
	if p.health != nil {
		stats.Accumulated = len(p.proxies)
		for _, h := range p.health {
			if h.Dead {
				stats.DeadCount++
			} else {
				stats.Live++
			}
		}
	}

	return stats
}

// AddProxies appends new proxies to a pool's accumulated list, deduplicating.
// Returns the number of newly added proxies and the new total.
func (s *Store) AddProxies(name PoolName, newProxies model.ProxyList) (added int, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)

	seen := make(map[string]struct{}, len(p.proxies))
	for _, proxy := range p.proxies {
		seen[proxy.Key()] = struct{}{}
	}

	for _, proxy := range newProxies {
		key := proxy.Key()
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			p.proxies = append(p.proxies, proxy)
			added++
		}
	}

	return added, len(p.proxies)
}

// GetProxies returns a copy of the accumulated proxy list for a pool.
func (s *Store) GetProxies(name PoolName) model.ProxyList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.getPool(name)
	out := make(model.ProxyList, len(p.proxies))
	copy(out, p.proxies)
	return out
}

// ClearProxies clears the accumulated proxy list and results for a pool.
func (s *Store) ClearProxies(name PoolName) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)
	p.proxies = nil
	p.results = nil
	p.health = nil
	p.updatedAt = time.Time{}
}

// MergeAndRevive merges scraped proxies into the accumulated pool (deduplicated).
// Dead proxies that reappear in the scraped list are revived (Dead=false, ConsecFails=0).
// Returns the number of newly added and revived proxies.
func (s *Store) MergeAndRevive(name PoolName, scraped model.ProxyList) (added, revived int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)
	now := time.Now()

	if p.health == nil {
		p.health = make(map[string]*ProxyHealth)
	}

	seen := make(map[string]struct{}, len(p.proxies))
	for _, proxy := range p.proxies {
		seen[proxy.Key()] = struct{}{}
	}

	for _, proxy := range scraped {
		key := proxy.Key()
		if h, exists := p.health[key]; exists {
			// Existing proxy — update LastSeen; revive if dead
			h.LastSeen = now
			if h.Dead {
				h.Dead = false
				h.ConsecFails = 0
				h.RetestFails = 0
				h.RetestDate = ""
				revived++
			}
		} else {
			// Brand new proxy
			p.health[key] = &ProxyHealth{
				FirstSeen: now,
				LastSeen:  now,
			}
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			p.proxies = append(p.proxies, proxy)
			added++
		}
	}

	return added, revived
}

// GetTestableProxies returns proxies that should be tested this cycle:
// - All live (non-dead) proxies
// - Dead proxies eligible for retest: every 3 hours, max 3 failures per day
func (s *Store) GetTestableProxies(name PoolName) model.ProxyList {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.getPool(name)
	now := time.Now()
	today := now.Format("2006-01-02")

	var out model.ProxyList
	for _, proxy := range p.proxies {
		h := p.health[proxy.Key()]
		if h == nil || !h.Dead {
			// Live proxy — always test
			out = append(out, proxy)
		} else {
			// Dead proxy — retest if: quota not exhausted AND 3h since last test
			quotaOK := h.RetestDate != today || h.RetestFails < 3
			cooldownOK := h.LastTested.IsZero() || now.Sub(h.LastTested) >= 3*time.Hour
			if quotaOK && cooldownOK {
				out = append(out, proxy)
			}
		}
	}
	return out
}

// UpdateHealth updates proxy health based on test results.
// Live proxies: 10 consecutive failures → dead.
// Dead proxies being retested: success → revive; failure → increment daily retest counter.
func (s *Store) UpdateHealth(name PoolName, results []model.TestResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)
	now := time.Now()
	today := now.Format("2006-01-02")

	if p.health == nil {
		p.health = make(map[string]*ProxyHealth)
	}

	for _, r := range results {
		key := r.Proxy.Key()
		h, exists := p.health[key]
		if !exists {
			h = &ProxyHealth{FirstSeen: now, LastSeen: now}
			p.health[key] = h
		}
		h.LastTested = now

		if h.Dead {
			// Dead proxy being retested
			if r.Success {
				// Revive
				h.Dead = false
				h.ConsecFails = 0
				h.RetestFails = 0
				h.RetestDate = ""
			} else {
				// Track daily retest failures
				if h.RetestDate != today {
					h.RetestDate = today
					h.RetestFails = 1
				} else {
					h.RetestFails++
				}
			}
		} else {
			// Live proxy
			if r.Success {
				h.ConsecFails = 0
			} else {
				h.ConsecFails++
				if h.ConsecFails >= 10 {
					h.Dead = true
					h.RetestFails = 0
					h.RetestDate = ""
				}
			}
		}
	}
}
