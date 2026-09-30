// Package config loads the service configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Default values used when the corresponding environment variable is unset.
const (
	DefaultAddr           = ":8080"
	DefaultHTTPAddr       = ":80"
	DefaultHTTPSAddr      = ":443"
	DefaultImagesDir      = "./images"
	DefaultCopyright      = "Wallpaper Collection"
	DefaultTimezone       = "Local"
	DefaultRescanInterval = 60 * time.Second
	DefaultLogLevel       = "info"
	DefaultACMECacheDir   = "./certs"

	DefaultSource        = "local"
	DefaultGitHubRef     = "master"
	DefaultGitHubPath    = "images"
	DefaultGitHubAPIBase = "https://api.github.com"
	DefaultGitHubRawBase = "https://raw.githubusercontent.com"
	DefaultGitHubRefresh = 15 * time.Minute

	// SourceLocal serves images from a local directory.
	SourceLocal = "local"
	// SourceGitHub serves images stored in a GitHub repository, proxied
	// through this service.
	SourceGitHub = "github"
)

// Config holds the resolved runtime configuration.
type Config struct {
	Addr           string
	ImagesDir      string
	BaseURL        string
	Copyright      string
	Timezone       string
	Location       *time.Location
	RescanInterval time.Duration
	LogLevel       slog.Level

	// Domains holds the ACME host names. When empty the service runs in plain
	// HTTP (IP) mode; when set it serves HTTPS with automatically issued
	// Let's Encrypt certificates.
	Domains      []string
	ACMEEmail    string
	ACMECacheDir string
	ACMEStaging  bool
	HTTPAddr     string
	HTTPSAddr    string

	// Source selects where wallpapers come from: "local" or "github".
	Source string
	// GitHub holds the repository settings used when Source is "github".
	GitHubRepo            string
	GitHubRef             string
	GitHubPath            string
	GitHubToken           string
	GitHubAPIBase         string
	GitHubRawBase         string
	GitHubRefreshInterval time.Duration
}

// TLS reports whether the service should run in HTTPS/ACME mode.
func (c Config) TLS() bool { return len(c.Domains) > 0 }

// Load reads the configuration from environment variables and applies defaults.
func Load() Config {
	repo := getString("WALLPAPER_GITHUB_REPO", "")
	source := strings.ToLower(strings.TrimSpace(os.Getenv("WALLPAPER_SOURCE")))
	// Setting only a repository implies GitHub mode; otherwise default to the
	// self-contained local directory source.
	if source == "" {
		if repo != "" {
			source = SourceGitHub
		} else {
			source = DefaultSource
		}
	}

	cfg := Config{
		Addr:           getString("WALLPAPER_ADDR", DefaultAddr),
		ImagesDir:      getString("WALLPAPER_IMAGES_DIR", DefaultImagesDir),
		BaseURL:        strings.TrimRight(getString("WALLPAPER_BASE_URL", ""), "/"),
		Copyright:      getString("WALLPAPER_COPYRIGHT", DefaultCopyright),
		Timezone:       getString("WALLPAPER_TIMEZONE", DefaultTimezone),
		RescanInterval: getDuration("WALLPAPER_RESCAN_INTERVAL", DefaultRescanInterval),
		LogLevel:       parseLevel(getString("WALLPAPER_LOG_LEVEL", DefaultLogLevel)),
		Domains:        parseDomains(getString("WALLPAPER_DOMAIN", "")),
		ACMEEmail:      getString("WALLPAPER_ACME_EMAIL", ""),
		ACMECacheDir:   getString("WALLPAPER_ACME_CACHE_DIR", DefaultACMECacheDir),
		ACMEStaging:    getBool("WALLPAPER_ACME_STAGING", false),
		HTTPAddr:       getString("WALLPAPER_HTTP_ADDR", DefaultHTTPAddr),
		HTTPSAddr:      getString("WALLPAPER_HTTPS_ADDR", DefaultHTTPSAddr),
		Source:         parseSource(source),

		GitHubRepo:            repo,
		GitHubRef:             getString("WALLPAPER_GITHUB_REF", DefaultGitHubRef),
		GitHubPath:            getString("WALLPAPER_GITHUB_PATH", DefaultGitHubPath),
		GitHubToken:           getString("WALLPAPER_GITHUB_TOKEN", ""),
		GitHubAPIBase:         getString("WALLPAPER_GITHUB_API_BASE", DefaultGitHubAPIBase),
		GitHubRawBase:         getString("WALLPAPER_GITHUB_RAW_BASE", DefaultGitHubRawBase),
		GitHubRefreshInterval: getDuration("WALLPAPER_GITHUB_REFRESH_INTERVAL", DefaultGitHubRefresh),
	}
	cfg.Location = loadLocation(cfg.Timezone)

	// In domain mode the public base URL is derived from the primary domain
	// unless it was set explicitly.
	if cfg.TLS() && cfg.BaseURL == "" {
		cfg.BaseURL = "https://" + cfg.Domains[0]
	}
	return cfg
}

// UsesGitHub reports whether wallpapers are read from a GitHub repository.
func (c Config) UsesGitHub() bool { return c.Source == SourceGitHub }

// Validate reports configuration problems that would prevent the service from
// running.
func (c Config) Validate() error {
	switch c.Source {
	case SourceLocal, SourceGitHub:
	default:
		return fmt.Errorf("WALLPAPER_SOURCE must be %q or %q, got %q", SourceLocal, SourceGitHub, c.Source)
	}
	if c.UsesGitHub() && strings.TrimSpace(c.GitHubRepo) == "" {
		return fmt.Errorf("WALLPAPER_GITHUB_REPO is required when WALLPAPER_SOURCE=%s", SourceGitHub)
	}
	return nil
}

func parseSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SourceGitHub, "git", "github.com":
		return SourceGitHub
	default:
		return SourceLocal
	}
}

func getString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return d
}

func getBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func loadLocation(name string) *time.Location {
	if name == "" || strings.EqualFold(name, "Local") {
		return time.Local
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}

func parseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// parseDomains splits a comma-separated host list, normalizes each entry and
// removes duplicates while preserving order.
func parseDomains(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := make(map[string]bool)
	out := make([]string, 0, 2)
	for _, part := range strings.Split(raw, ",") {
		host := normalizeHost(part)
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeHost accepts either a bare host or a full URL and returns the host
// portion in lower case.
func normalizeHost(raw string) string {
	host := strings.TrimSpace(strings.ToLower(raw))
	if host == "" {
		return ""
	}
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	if idx := strings.IndexAny(host, "/?#"); idx >= 0 {
		host = host[:idx]
	}
	return strings.TrimSuffix(host, "/")
}

// String renders the configuration for startup logging, hiding nothing sensitive.
func (c Config) String() string {
	mode := "http(ip)"
	if c.TLS() {
		mode = "https(acme)"
	}
	base := fmt.Sprintf("mode=%s source=%s addr=%s base_url=%q timezone=%s log_level=%s",
		mode, c.Source, c.Addr, c.BaseURL, c.Timezone, c.LogLevel)
	if c.UsesGitHub() {
		return base + fmt.Sprintf(" github_repo=%s github_ref=%s github_path=%s github_refresh=%s",
			c.GitHubRepo, c.GitHubRef, c.GitHubPath, c.GitHubRefreshInterval)
	}
	return base + fmt.Sprintf(" images_dir=%s rescan=%s http_addr=%s https_addr=%s domains=%v",
		c.ImagesDir, c.RescanInterval, c.HTTPAddr, c.HTTPSAddr, c.Domains)
}
