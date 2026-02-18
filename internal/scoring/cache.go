package scoring

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cachedScore represents the cached scoring result for a single IP.
type cachedScore struct {
	Date        string `json:"date"`                  // "2006-01-02"
	IPQS        *int   `json:"ipqs,omitempty"`
	Scamalytics *int   `json:"scamalytics,omitempty"`
	AbuseIPDB   *int   `json:"abuseipdb,omitempty"`
}

// scoreCache is a thread-safe in-memory cache backed by a JSON file.
type scoreCache struct {
	mu      sync.RWMutex
	entries map[string]cachedScore // key = IP address
	path    string
}

// defaultCachePath returns ~/.proxyyopick/score_cache.json
func defaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".proxyyopick", "score_cache.json")
}

// loadCache loads the cache from disk, or returns an empty cache.
func loadCache(path string) *scoreCache {
	if path == "" {
		path = defaultCachePath()
	}

	c := &scoreCache{
		entries: make(map[string]cachedScore),
		path:    path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read score cache", "path", path, "error", err)
		}
		return c
	}

	if err := json.Unmarshal(data, &c.entries); err != nil {
		slog.Warn("failed to parse score cache, starting fresh", "error", err)
		c.entries = make(map[string]cachedScore)
		return c
	}

	// Prune entries older than 7 days
	now := time.Now()
	for ip, entry := range c.entries {
		entryDate, err := time.Parse("2006-01-02", entry.Date)
		if err != nil || now.Sub(entryDate) > 7*24*time.Hour {
			delete(c.entries, ip)
		}
	}

	slog.Info("loaded score cache", "path", path, "entries", len(c.entries))
	return c
}

func (c *scoreCache) get(ip string) (cachedScore, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[ip]
	return entry, ok
}

func (c *scoreCache) set(ip string, entry cachedScore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[ip] = entry
}

// save writes the cache to disk atomically, creating parent directories if needed.
func (c *scoreCache) save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tmpPath := c.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write cache temp: %w", err)
	}
	if err := os.Rename(tmpPath, c.path); err != nil {
		return fmt.Errorf("rename cache: %w", err)
	}

	slog.Info("saved score cache", "path", c.path, "entries", len(c.entries))
	return nil
}
