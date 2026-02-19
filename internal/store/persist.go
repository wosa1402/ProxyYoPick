package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/efan/proxyyopick/internal/model"
)

// poolSnapshot is the JSON-serializable representation of a pool's state.
type poolSnapshot struct {
	Proxies   model.ProxyList         `json:"proxies"`
	Health    map[string]*ProxyHealth `json:"health"`
	Results   []model.TestResult      `json:"results"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// PoolSnapshot is the exported version used for data export/import via the web UI.
type PoolSnapshot struct {
	Proxies   model.ProxyList         `json:"proxies"`
	Health    map[string]*ProxyHealth `json:"health"`
	Results   []model.TestResult      `json:"results"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// ExportPool returns a deep copy of the pool's full state for export.
func (s *Store) ExportPool(name PoolName) *PoolSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.getPool(name)

	proxies := make(model.ProxyList, len(p.proxies))
	copy(proxies, p.proxies)

	health := make(map[string]*ProxyHealth, len(p.health))
	for k, v := range p.health {
		h := *v
		health[k] = &h
	}

	results := make([]model.TestResult, len(p.results))
	copy(results, p.results)

	return &PoolSnapshot{
		Proxies:   proxies,
		Health:    health,
		Results:   results,
		UpdatedAt: p.updatedAt,
	}
}

// ImportPool replaces a pool's state with the given snapshot.
// Rebuilds TestResult.Latency from LatencyMs (same as Load).
func (s *Store) ImportPool(name PoolName, snap *PoolSnapshot) {
	for i := range snap.Results {
		snap.Results[i].Latency = time.Duration(snap.Results[i].LatencyMs) * time.Millisecond
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.getPool(name)
	p.proxies = snap.Proxies
	p.health = snap.Health
	p.results = snap.Results
	p.updatedAt = snap.UpdatedAt
}

// defaultDataDir returns ~/.proxyyopick/
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".proxyyopick")
}

// poolFileName returns the persistence file name for a pool.
func poolFileName(name PoolName) string {
	return fmt.Sprintf("pool_%s.json", name)
}

// Save persists all non-empty pools to disk as JSON files.
// If dir is empty, defaults to ~/.proxyyopick/.
func (s *Store) Save(dir string) {
	if dir == "" {
		dir = defaultDataDir()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, p := range s.pools {
		if len(p.proxies) == 0 && len(p.results) == 0 {
			// Remove stale file if pool is empty (e.g. after clear)
			path := filepath.Join(dir, poolFileName(name))
			os.Remove(path)
			continue
		}

		snap := poolSnapshot{
			Proxies:   p.proxies,
			Health:    p.health,
			Results:   p.results,
			UpdatedAt: p.updatedAt,
		}

		if err := saveSnapshot(dir, name, &snap); err != nil {
			slog.Error("failed to save pool", "pool", name, "error", err)
		} else {
			slog.Info("saved pool to disk", "pool", name, "proxies", len(snap.Proxies), "results", len(snap.Results))
		}
	}
}

// saveSnapshot writes a single pool snapshot to disk atomically.
func saveSnapshot(dir string, name PoolName, snap *poolSnapshot) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pool %s: %w", name, err)
	}

	path := filepath.Join(dir, poolFileName(name))
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// Load restores pool data from disk. If dir is empty, defaults to ~/.proxyyopick/.
// Failures are logged but do not prevent the store from starting (graceful fallback).
func (s *Store) Load(dir string) {
	if dir == "" {
		dir = defaultDataDir()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range []PoolName{PoolAuto, PoolManual} {
		path := filepath.Join(dir, poolFileName(name))

		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("failed to read pool file", "pool", name, "path", path, "error", err)
			}
			continue
		}

		var snap poolSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			slog.Warn("failed to parse pool file, starting fresh", "pool", name, "error", err)
			continue
		}

		// Rebuild TestResult.Latency from LatencyMs (Latency has json:"-")
		for i := range snap.Results {
			snap.Results[i].Latency = time.Duration(snap.Results[i].LatencyMs) * time.Millisecond
		}

		p := s.getPool(name)
		p.proxies = snap.Proxies
		p.health = snap.Health
		p.results = snap.Results
		p.updatedAt = snap.UpdatedAt

		slog.Info("loaded pool from disk", "pool", name, "proxies", len(p.proxies), "results", len(p.results))
	}
}
