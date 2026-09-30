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
}

// TLS reports whether the service should run in HTTPS/ACME mode.
func (c Config) TLS() bool { return len(c.Domains) > 0 }

// Load reads the configuration from environment variables and applies defaults.
func Load() Config {
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
	}
	cfg.Location = loadLocation(cfg.Timezone)

	// In domain mode the public base URL is derived from the primary domain
	// unless it was set explicitly.
	if cfg.TLS() && cfg.BaseURL == "" {
		cfg.BaseURL = "https://" + cfg.Domains[0]
	}
	return cfg
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
	return fmt.Sprintf("mode=%s addr=%s http_addr=%s https_addr=%s domains=%v images_dir=%s base_url=%q timezone=%s rescan=%s log_level=%s",
		mode, c.Addr, c.HTTPAddr, c.HTTPSAddr, c.Domains, c.ImagesDir, c.BaseURL, c.Timezone, c.RescanInterval, c.LogLevel)
}
