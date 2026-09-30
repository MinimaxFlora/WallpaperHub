// Package server implements the native REST wallpaper API.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"wallpaper-api/internal/catalog"
	"wallpaper-api/internal/config"
	"wallpaper-api/internal/errs"
	"wallpaper-api/internal/manifest"
	"wallpaper-api/internal/store"
)

// Server wires the HTTP handlers to the configuration, manifest catalogue and
// image store.
type Server struct {
	cfg     config.Config
	catalog *catalog.Catalog
	store   store.Store
	logger  *slog.Logger
	now     func() time.Time
	limiter *rateLimiter
}

// New creates a Server.
func New(cfg config.Config, cat *catalog.Catalog, st store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{cfg: cfg, catalog: cat, store: st, logger: logger, now: time.Now}
	if cfg.RateLimitLimit > 0 && cfg.RateLimitWindow > 0 {
		s.limiter = newRateLimiter(cfg.RateLimitLimit, cfg.RateLimitWindow)
	}
	return s
}

// Handler returns the fully decorated HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /v1/random", s.handleRandom)
	mux.HandleFunc("GET /v1/images", s.handleList)
	mux.HandleFunc("GET /v1/images/{id}", s.handleImageMeta)
	mux.HandleFunc("GET /v1/images/{id}/file", s.handleImageFile)
	mux.HandleFunc("GET /v1/tags", s.handleTags)
	mux.HandleFunc("GET /v1/categories", s.handleCategories)
	return s.withRecover(s.withLogging(s.withCORS(s.withRateLimit(mux))))
}

// manifestOrFail returns the current manifest or writes a 503 response.
func (s *Server) manifestOrFail(w http.ResponseWriter) (*manifest.Manifest, bool) {
	current := s.catalog.Snapshot()
	if current == nil {
		s.writeErr(w, errs.Unavailable("manifest_unavailable", "the manifest is not available yet"))
		return nil, false
	}
	return current, true
}
