// Package server implements the HTTP API and static image delivery.
package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wallpaper-api/internal/bing"
	"wallpaper-api/internal/config"
	"wallpaper-api/internal/index"
	"wallpaper-api/internal/selector"
)

// Server wires the HTTP handlers to the configuration and image index.
type Server struct {
	cfg      config.Config
	index    *index.Index
	renderer bing.Renderer
	logger   *slog.Logger
	now      func() time.Time
}

// New creates a Server.
func New(cfg config.Config, idx *index.Index, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		cfg:      cfg,
		index:    idx,
		renderer: bing.Renderer{Copyright: cfg.Copyright},
		logger:   logger,
		now:      time.Now,
	}
}

// Handler returns the fully decorated HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/HPImageArchive.aspx", s.handleArchive)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/images/", s.handleImage)
	return s.withCORS(s.withLogging(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	idx := parseOffset(r.URL.Query().Get("idx"))
	n := parseCount(r.URL.Query().Get("n"))

	images := s.index.Snapshot()
	resp := bing.Response{Images: make([]bing.Image, 0)}

	if len(images) > 0 {
		today := selector.Today(s.now(), s.cfg.Location)
		baseURL := s.baseURL(r)
		limit := min(n, len(images))
		for i := 0; i < limit; i++ {
			date := selector.OffsetDate(today, idx+i)
			pick := selector.PickIndex(date, s.cfg.Location, len(images))
			img := images[pick]
			resp.Images = append(resp.Images, s.renderer.Render(baseURL, img.RelPath, img.Name, date))
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resp); err != nil {
		s.logger.Error("encode response", "error", err)
	}
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/images/")
	rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}

	root, err := filepath.Abs(s.cfg.ImagesDir)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	absFull, err := filepath.Abs(full)
	if err != nil || (absFull != root && !strings.HasPrefix(absFull, root+string(os.PathSeparator))) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(absFull)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, absFull)
}

func (s *Server) baseURL(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return scheme + "://" + host
}

func parseOffset(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func parseCount(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v < 1 {
		return 1
	}
	return v
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}
