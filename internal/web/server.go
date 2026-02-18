package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/efan/proxyyopick/internal/geo"
	"github.com/efan/proxyyopick/internal/model"
	"github.com/efan/proxyyopick/internal/scoring"
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
	ScoreCfg    scoring.Config
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
	mux.HandleFunc("/api/import", s.handleImport)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// Start background scheduler for auto pool
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
	s.runPoolTest(ctx, store.PoolAuto)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runPoolTest(ctx, store.PoolAuto)
		}
	}
}

// runPoolTest scrapes and tests proxies for the auto pool.
func (s *Server) runPoolTest(ctx context.Context, pool store.PoolName) {
	if s.store.IsRunning(pool) {
		slog.Info("test already running, skipping", "pool", pool)
		return
	}

	s.store.SetRunning(pool, true)
	defer s.store.SetRunning(pool, false)

	slog.Info("starting proxy test", "pool", pool)

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

	results := s.testProxies(ctx, proxies)
	s.store.SetResults(pool, results)
	slog.Info("proxy test completed", "pool", pool, "total", len(results))
}

// runManualTest tests a given proxy list and stores results in the manual pool.
func (s *Server) runManualTest(ctx context.Context, proxies model.ProxyList) {
	if s.store.IsRunning(store.PoolManual) {
		slog.Info("manual test already running, skipping")
		return
	}

	s.store.SetRunning(store.PoolManual, true)
	defer s.store.SetRunning(store.PoolManual, false)

	slog.Info("starting manual proxy test", "count", len(proxies))

	results := s.testProxies(ctx, proxies)
	s.store.SetResults(store.PoolManual, results)
	slog.Info("manual proxy test completed", "total", len(results))
}

// testProxies is the shared test + geo lookup logic.
func (s *Server) testProxies(ctx context.Context, proxies model.ProxyList) []model.TestResult {
	t := tester.NewSOCKS5Tester(s.cfg.TargetURL, s.cfg.Timeout)
	results := tester.RunPool(ctx, t, proxies, tester.PoolConfig{Workers: s.cfg.Concurrency}, nil)
	results = tester.SortByLatency(results)

	proxyList := make(model.ProxyList, len(results))
	for i := range results {
		proxyList[i] = results[i].Proxy
	}
	geo.LookupCountries(ctx, proxyList)

	// IP scoring enrichment
	scoring.ScoreProxies(ctx, proxyList, s.cfg.ScoreCfg)

	for i := range results {
		results[i].Proxy = proxyList[i]
	}

	return results
}

// poolFromQuery extracts the pool name from the query parameter, defaults to "auto".
func poolFromQuery(r *http.Request) store.PoolName {
	p := r.URL.Query().Get("pool")
	if p == "manual" {
		return store.PoolManual
	}
	return store.PoolAuto
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	pool := poolFromQuery(r)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.store.GetStats(pool))
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	pool := poolFromQuery(r)
	results, _ := s.store.GetResults(pool)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pool := poolFromQuery(r)
	if s.store.IsRunning(pool) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_running"})
		return
	}

	go s.runPoolTest(s.ctx, pool)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleImport accepts proxy text (ip:port per line) via POST body or file upload.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.store.IsRunning(store.PoolManual) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "手动池测试正在运行中",
		})
		return
	}

	var textData string
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// File upload
		r.ParseMultipartForm(32 << 20) // 32MB max
		file, _, err := r.FormFile("file")
		if err != nil {
			// Fallback: try text field
			textData = r.FormValue("text")
		} else {
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "read file failed", http.StatusBadRequest)
				return
			}
			textData = string(data)
		}
		// Also append text field if both provided
		if extra := r.FormValue("text"); extra != "" && textData != "" {
			textData = textData + "\n" + extra
		} else if extra != "" {
			textData = extra
		}
	} else {
		// Plain text body
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		textData = string(data)
	}

	if strings.TrimSpace(textData) == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "未提供任何代理数据",
		})
		return
	}

	// Parse proxies from text
	src := source.NewTextSource(strings.NewReader(textData), "web-import")
	proxies, err := src.Fetch(s.ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}
	proxies = source.Deduplicate(proxies)

	if len(proxies) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "未解析到有效代理（格式: ip:port，每行一个）",
		})
		return
	}

	// Start test in background
	go s.runManualTest(s.ctx, proxies)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "started",
		"count":  len(proxies),
	})
}
