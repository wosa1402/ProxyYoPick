package store

import (
	"sync"
	"time"

	"github.com/efan/proxyyopick/internal/model"
)

// Store holds test results in memory with thread-safe access.
type Store struct {
	mu        sync.RWMutex
	results   []model.TestResult
	updatedAt time.Time
	running   bool
}

func New() *Store {
	return &Store{}
}

func (s *Store) SetResults(results []model.TestResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = results
	s.updatedAt = time.Now()
}

func (s *Store) GetResults() ([]model.TestResult, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.TestResult, len(s.results))
	copy(out, s.results)
	return out, s.updatedAt
}

func (s *Store) SetRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = running
}

func (s *Store) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Stats returns summary statistics.
type Stats struct {
	Total     int     `json:"total"`
	Success   int     `json:"success"`
	Fail      int     `json:"fail"`
	AvgMs     int64   `json:"avg_ms"`
	FastestMs int64   `json:"fastest_ms"`
	UpdatedAt string  `json:"updated_at"`
	Running   bool    `json:"running"`
}

func (s *Store) GetStats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{
		Total:   len(s.results),
		Running: s.running,
	}

	if s.updatedAt.IsZero() {
		stats.UpdatedAt = ""
	} else {
		stats.UpdatedAt = s.updatedAt.Format("2006-01-02 15:04:05")
	}

	var totalMs int64
	stats.FastestMs = -1
	for _, r := range s.results {
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
