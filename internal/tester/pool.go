package tester

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/efan/proxyyopick/internal/model"
)

// PoolConfig controls the worker pool behavior.
type PoolConfig struct {
	Workers int
}

// ProgressFunc is called when a proxy test completes.
// done is the number of completed tests, total is the total number of proxies.
type ProgressFunc func(done, total int)

// RunPool tests all proxies concurrently using a fixed worker pool.
func RunPool(ctx context.Context, t Tester, proxies model.ProxyList, cfg PoolConfig, onProgress ProgressFunc) []model.TestResult {
	if len(proxies) == 0 {
		return nil
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = 100
	}
	if workers > len(proxies) {
		workers = len(proxies)
	}

	jobs := make(chan model.Proxy, workers*2)
	results := make(chan model.TestResult, workers*2)

	var wg sync.WaitGroup
	var doneCount atomic.Int64
	total := len(proxies)

	// Spawn workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					result := t.Test(ctx, p)
					results <- result
					done := int(doneCount.Add(1))
					if onProgress != nil {
						onProgress(done, total)
					}
				}
			}
		}()
	}

	// Feeder
	go func() {
		defer close(jobs)
		for _, p := range proxies {
			select {
			case <-ctx.Done():
				return
			case jobs <- p:
			}
		}
	}()

	// Closer
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collector
	out := make([]model.TestResult, 0, total)
	for r := range results {
		out = append(out, r)
	}

	if onProgress != nil {
		onProgress(total, total)
		fmt.Println() // newline after progress
	}

	return out
}
