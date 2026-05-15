package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgSchema = `CREATE TABLE IF NOT EXISTS pool_snapshots (
    name       TEXT PRIMARY KEY,
    data       JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

var (
	pgPool     *pgxpool.Pool
	pgPoolOnce sync.Once
	pgPoolErr  error
)

// pgEnabled reports whether DATABASE_URL is configured.
func pgEnabled() bool {
	return os.Getenv("DATABASE_URL") != ""
}

// getPGPool lazily initializes a connection pool and ensures the schema exists.
func getPGPool() (*pgxpool.Pool, error) {
	pgPoolOnce.Do(func() {
		url := os.Getenv("DATABASE_URL")
		if url == "" {
			pgPoolErr = errors.New("DATABASE_URL not set")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			pgPoolErr = fmt.Errorf("parse DATABASE_URL: %w", err)
			return
		}
		cfg.MaxConns = 4
		cfg.MinConns = 0
		cfg.MaxConnIdleTime = 5 * time.Minute

		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			pgPoolErr = fmt.Errorf("connect db: %w", err)
			return
		}

		if _, err := pool.Exec(ctx, pgSchema); err != nil {
			pool.Close()
			pgPoolErr = fmt.Errorf("init schema: %w", err)
			return
		}

		pgPool = pool
		slog.Info("postgres persistence enabled")
	})
	return pgPool, pgPoolErr
}

// saveToPG writes every non-empty pool as a single JSONB row.
// Empty pools (after clear) are deleted to mirror the file-based behavior.
func (s *Store) saveToPG() error {
	pool, err := getPGPool()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, p := range s.pools {
		if len(p.proxies) == 0 && len(p.results) == 0 {
			if _, err := pool.Exec(ctx, `DELETE FROM pool_snapshots WHERE name = $1`, string(name)); err != nil {
				slog.Error("pg delete empty pool failed", "pool", name, "error", err)
			}
			continue
		}

		snap := poolSnapshot{
			Proxies:   p.proxies,
			Health:    p.health,
			Results:   p.results,
			UpdatedAt: p.updatedAt,
		}
		data, err := json.Marshal(snap)
		if err != nil {
			slog.Error("marshal pool failed", "pool", name, "error", err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO pool_snapshots (name, data, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (name) DO UPDATE
			   SET data = EXCLUDED.data, updated_at = NOW()`,
			string(name), data)
		if err != nil {
			slog.Error("pg save failed", "pool", name, "error", err)
			continue
		}
		slog.Info("saved pool to pg", "pool", name, "proxies", len(snap.Proxies), "results", len(snap.Results))
	}
	return nil
}

// loadFromPG restores both pools from the database. Missing rows are skipped silently.
func (s *Store) loadFromPG() error {
	pool, err := getPGPool()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, name := range []PoolName{PoolAuto, PoolManual} {
		var data []byte
		err := pool.QueryRow(ctx, `SELECT data FROM pool_snapshots WHERE name = $1`, string(name)).Scan(&data)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			slog.Warn("pg load failed", "pool", name, "error", err)
			continue
		}

		var snap poolSnapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			slog.Warn("pg parse snapshot failed, starting fresh", "pool", name, "error", err)
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

		slog.Info("loaded pool from pg", "pool", name, "proxies", len(p.proxies), "results", len(p.results))
	}
	return nil
}
