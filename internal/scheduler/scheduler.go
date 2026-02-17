package scheduler

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
	"time"
)

// Scheduler runs a function on a fixed interval with graceful shutdown.
type Scheduler struct {
	Interval time.Duration
}

func New(interval time.Duration) *Scheduler {
	return &Scheduler{Interval: interval}
}

// Start runs fn immediately, then on every tick until interrupted.
func (s *Scheduler) Start(ctx context.Context, fn func() error) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("scheduler started", "interval", s.Interval)

	// Run immediately
	if err := fn(); err != nil {
		slog.Error("run failed", "error", err)
	}

	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopped")
			return nil
		case <-ticker.C:
			slog.Info("scheduled run starting")
			if err := fn(); err != nil {
				slog.Error("scheduled run failed", "error", err)
			}
		}
	}
}
