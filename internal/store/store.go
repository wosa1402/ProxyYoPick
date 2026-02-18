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

// pool holds results and state for a single proxy pool.
type pool struct {
	results   []model.TestResult
	updatedAt time.Time
	running   bool
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
	Total     int    `json:"total"`
	Success   int    `json:"success"`
	Fail      int    `json:"fail"`
	AvgMs     int64  `json:"avg_ms"`
	FastestMs int64  `json:"fastest_ms"`
	UpdatedAt string `json:"updated_at"`
	Running   bool   `json:"running"`
	Pool      string `json:"pool"`
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

	return stats
}
