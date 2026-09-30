// Command wallpaper-api serves a self-hosted wallpaper REST API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wallpaper-api/internal/catalog"
	"wallpaper-api/internal/config"
	"wallpaper-api/internal/server"
	"wallpaper-api/internal/store"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	logger.Info("starting wallpaper-api", "config", cfg.String())

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
	logger.Info("stopped")
}

func run(cfg config.Config, logger *slog.Logger) error {
	images, err := buildStore(cfg, logger)
	if err != nil {
		return err
	}
	cat := catalog.New(manifestFetcher(cfg))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	refreshManifest(ctx, cat, cfg, logger)
	stopRefresh := startManifestRefresh(cat, cfg, logger)
	defer stopRefresh()

	handler := server.New(cfg, cat, images, logger).Handler()
	return runHTTP(ctx, cfg, handler, logger)
}

// buildStore selects the image backend for the configured source.
func buildStore(cfg config.Config, logger *slog.Logger) (store.Store, error) {
	if cfg.UsesGitHub() {
		return store.NewGitHub(store.GitHubConfig{
			RawBase:   cfg.GitHubRawBase,
			Repo:      cfg.GitHubRepo,
			Ref:       cfg.GitHubRef,
			Token:     cfg.GitHubToken,
			UserAgent: "wallpaper-api",
			CacheDir:  cfg.CacheDir,
			Logger:    logger,
		})
	}
	return store.NewLocal(cfg.Root), nil
}

// manifestFetcher selects how the manifest is obtained.
func manifestFetcher(cfg config.Config) catalog.Fetcher {
	if cfg.UsesGitHub() || cfg.ManifestURL != "" {
		return catalog.HTTPFetcher{URL: cfg.GitHubManifestURL(), Token: cfg.GitHubToken}
	}
	return catalog.FileFetcher{Path: cfg.ManifestPath}
}

func refreshManifest(ctx context.Context, cat *catalog.Catalog, cfg config.Config, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	count, err := cat.Refresh(ctx)
	if err != nil {
		logger.Warn("initial manifest load failed; starting without a manifest", "error", err)
		return
	}
	logger.Info("manifest loaded", "images", count)
}

// startManifestRefresh reloads the manifest periodically and returns a stop
// function.
func startManifestRefresh(cat *catalog.Catalog, cfg config.Config, logger *slog.Logger) func() {
	if cfg.ManifestRefreshInterval <= 0 {
		logger.Info("manifest refresh disabled", "interval", cfg.ManifestRefreshInterval)
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(cfg.ManifestRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				count, err := cat.Refresh(ctx)
				cancel()
				if err != nil {
					logger.Error("manifest refresh failed; keeping previous manifest", "error", err)
					continue
				}
				logger.Info("manifest refreshed", "images", count)
			}
		}
	}()
	return func() { close(stop) }
}

// runHTTP serves plain HTTP. TLS is terminated by the reverse proxy in front of
// the service.
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

func shutdown(logger *slog.Logger, srv *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
