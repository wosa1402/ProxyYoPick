package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/efan/proxyyopick/internal/geo"
	"github.com/efan/proxyyopick/internal/model"
	"github.com/efan/proxyyopick/internal/source"
	"github.com/efan/proxyyopick/internal/store"
	"github.com/efan/proxyyopick/internal/tester"
)

//go:embed static/*
var staticFiles embed.FS

// Config holds web server configuration.
type Config struct {
	Addr        string
	ScrapeURL   string
	Concurrency int
	Timeout     time.Duration
	TargetURL   string
	Interval    time.Duration
}

// Server is the web server with integrated scheduler.
type Server struct {
	cfg   Config
	store *store.Store
	ctx   context.Context
}

func NewServer(cfg Config) *Server {
	return &Server{
		cfg:   cfg,
		store: store.New(),
	}
}

// Start starts the web server and the background scheduler.
func (s *Server) Start(ctx context.Context) error {
	s.ctx = ctx
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/results", s.handleResults)
	mux.HandleFunc("/api/trigger", s.handleTrigger)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// Start background scheduler
	go s.scheduler(ctx)

	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("web server starting", "addr", s.cfg.Addr)
	fmt.Printf("🌐 Web 仪表盘已启动: http://localhost%s\n", s.cfg.Addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) scheduler(ctx context.Context) {
	// Run immediately
	s.runTest(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runTest(ctx)
		}
	}
}

func (s *Server) runTest(ctx context.Context) {
	if s.store.IsRunning() {
		slog.Info("test already running, skipping")
		return
	}

	s.store.SetRunning(true)
	defer s.store.SetRunning(false)

	slog.Info("starting proxy test")

	// Fetch
	scraper := source.NewScraper(s.cfg.ScrapeURL)
	proxies, err := scraper.Fetch(ctx)
	if err != nil {
		slog.Error("scrape failed", "error", err)
		return
	}
	proxies = source.Deduplicate(proxies)
	if len(proxies) == 0 {
		slog.Warn("no proxies found")
		return
	}

	slog.Info("testing proxies", "count", len(proxies))

	// Test
	t := tester.NewSOCKS5Tester(s.cfg.TargetURL, s.cfg.Timeout)
	results := tester.RunPool(ctx, t, proxies, tester.PoolConfig{Workers: s.cfg.Concurrency}, nil)
	results = tester.SortByLatency(results)

	// Geo lookup
	proxyList := make(model.ProxyList, len(results))
	for i := range results {
		proxyList[i] = results[i].Proxy
	}
	geo.LookupCountries(ctx, proxyList)
	for i := range results {
		results[i].Proxy = proxyList[i]
	}

	s.store.SetResults(results)
	slog.Info("proxy test completed", "total", len(results))
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.store.GetStats())
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	results, _ := s.store.GetResults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.store.IsRunning() {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_running"})
		return
	}

	go s.runTest(s.ctx)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}
