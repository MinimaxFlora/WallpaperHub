// Command wallpaper-api serves a self-hosted, Bing-compatible wallpaper API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"wallpaper-api/internal/acme"
	"wallpaper-api/internal/config"
	"wallpaper-api/internal/index"
	"wallpaper-api/internal/server"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	logger.Info("starting wallpaper-api", "config", cfg.String())

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
	logger.Info("stopped")
}

func run(cfg config.Config, logger *slog.Logger) error {
	idx := index.New(cfg.ImagesDir)
	if count, err := idx.Refresh(); err != nil {
		logger.Warn("initial image scan failed; starting with an empty index",
			"dir", cfg.ImagesDir, "error", err)
	} else {
		logger.Info("image index built", "dir", cfg.ImagesDir, "count", count)
	}

	stopRescan := make(chan struct{})
	defer close(stopRescan)
	if cfg.RescanInterval > 0 {
		go rescanLoop(idx, cfg.RescanInterval, logger, stopRescan)
	} else {
		logger.Info("image rescan disabled", "interval", cfg.RescanInterval)
	}

	handler := server.New(cfg, idx, logger).Handler()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.TLS() {
		return runTLS(ctx, cfg, handler, logger)
	}
	return runHTTP(ctx, cfg, handler, logger)
}

// runHTTP serves plain HTTP. This is the default IP mode used when no domain
// is configured.
func runHTTP(ctx context.Context, cfg config.Config, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	logger.Info("listening", "mode", "http", "addr", cfg.Addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}
	return shutdown(logger, srv)
}

// runTLS serves HTTPS with automatically issued Let's Encrypt certificates and
// keeps a plain HTTP listener for ACME http-01 challenges and HTTPS redirects.
func runTLS(ctx context.Context, cfg config.Config, handler http.Handler, logger *slog.Logger) error {
	manager := acme.NewManager(cfg)
	logger.Info("acme enabled",
		"domains", cfg.Domains,
		"cache_dir", cfg.ACMECacheDir,
		"staging", cfg.ACMEStaging,
		"email_configured", cfg.ACMEEmail != "",
	)

	httpsSrv := &http.Server{
		Addr:              cfg.HTTPSAddr,
		Handler:           handler,
		TLSConfig:         manager.TLSConfig(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	// The HTTP listener answers ACME http-01 challenges and redirects
	// everything else to HTTPS.
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           manager.HTTPHandler(redirectToHTTPS()),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() {
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("https listener: %w", err)
		}
	}()
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http listener: %w", err)
		}
	}()
	logger.Info("listening",
		"mode", "https",
		"https_addr", cfg.HTTPSAddr,
		"http_addr", cfg.HTTPAddr,
		"public_url", cfg.BaseURL,
	)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	var wg sync.WaitGroup
	for _, srv := range []*http.Server{httpsSrv, httpSrv} {
		wg.Add(1)
		go func(s *http.Server) {
			defer wg.Done()
			if err := shutdown(logger, s); err != nil {
				logger.Error("graceful shutdown failed", "addr", s.Addr, "error", err)
			}
		}(srv)
	}
	wg.Wait()
	return nil
}

func shutdown(logger *slog.Logger, srv *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// redirectToHTTPS sends every request that is not an ACME challenge to the
// HTTPS endpoint of the same host and path.
func redirectToHTTPS() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

func rescanLoop(idx *index.Index, interval time.Duration, logger *slog.Logger, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			before := idx.Len()
			count, err := idx.Refresh()
			if err != nil {
				logger.Error("image rescan failed; keeping previous index",
					"error", err, "count", before)
				continue
			}
			if count != before {
				logger.Info("image index refreshed", "count", count)
			}
		}
	}
}
