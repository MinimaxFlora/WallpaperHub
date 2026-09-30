package config

import (
	"reflect"
	"testing"
)

func TestParseDomainsNormalizesAndDedupes(t *testing.T) {
	got := parseDomains(" Example.com , https://cdn.example.com/path , EXAMPLE.com ,, ")
	want := []string{"example.com", "cdn.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDomains() = %v, want %v", got, want)
	}
}

func TestParseDomainsEmpty(t *testing.T) {
	for _, raw := range []string{"", "   ", ",", " , "} {
		if got := parseDomains(raw); got != nil {
			t.Fatalf("parseDomains(%q) = %v, want nil", raw, got)
		}
	}
}

func TestDefaultIsIPMode(t *testing.T) {
	for _, key := range []string{
		"WALLPAPER_DOMAIN", "WALLPAPER_BASE_URL", "WALLPAPER_ADDR",
		"WALLPAPER_HTTP_ADDR", "WALLPAPER_HTTPS_ADDR", "WALLPAPER_ACME_EMAIL",
		"WALLPAPER_ACME_CACHE_DIR", "WALLPAPER_ACME_STAGING",
	} {
		t.Setenv(key, "")
	}
	cfg := Load()
	if cfg.TLS() {
		t.Fatal("expected plain HTTP mode when no domain is set")
	}
	if cfg.BaseURL != "" {
		t.Fatalf("BaseURL = %q, want empty (derived from request Host)", cfg.BaseURL)
	}
	if cfg.Addr != DefaultAddr {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, DefaultAddr)
	}
	if cfg.HTTPSAddr != DefaultHTTPSAddr || cfg.HTTPAddr != DefaultHTTPAddr {
		t.Fatalf("addrs = %q / %q", cfg.HTTPSAddr, cfg.HTTPAddr)
	}
	if cfg.ACMECacheDir != DefaultACMECacheDir {
		t.Fatalf("ACMECacheDir = %q, want %q", cfg.ACMECacheDir, DefaultACMECacheDir)
	}
}

func TestDomainModeEnablesTLSAndDerivesBaseURL(t *testing.T) {
	t.Setenv("WALLPAPER_DOMAIN", "wall.example.com")
	t.Setenv("WALLPAPER_BASE_URL", "")

	cfg := Load()
	if !cfg.TLS() {
		t.Fatal("expected TLS mode when a domain is set")
	}
	if got, want := cfg.Domains, []string{"wall.example.com"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Domains = %v, want %v", got, want)
	}
	if cfg.BaseURL != "https://wall.example.com" {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, "https://wall.example.com")
	}
}

func TestExplicitBaseURLWinsInDomainMode(t *testing.T) {
	t.Setenv("WALLPAPER_DOMAIN", "wall.example.com")
	t.Setenv("WALLPAPER_BASE_URL", "https://cdn.example.com/wall/")

	cfg := Load()
	if cfg.BaseURL != "https://cdn.example.com/wall" {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", cfg.BaseURL)
	}
}

func TestMultipleDomainsUseFirstForBaseURL(t *testing.T) {
	t.Setenv("WALLPAPER_DOMAIN", "a.example.com, b.example.com")
	t.Setenv("WALLPAPER_BASE_URL", "")

	cfg := Load()
	if len(cfg.Domains) != 2 {
		t.Fatalf("Domains = %v", cfg.Domains)
	}
	if cfg.BaseURL != "https://a.example.com" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
}

func TestBoolParsing(t *testing.T) {
	cases := map[string]bool{
		"1": true, "true": true, "TRUE": true, "yes": true, "on": true,
		"0": false, "false": false, "no": false, "off": false,
	}
	for raw, want := range cases {
		t.Setenv("WALLPAPER_ACME_STAGING", raw)
		if got := getBool("WALLPAPER_ACME_STAGING", !want); got != want {
			t.Fatalf("getBool(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestCustomAddresses(t *testing.T) {
	t.Setenv("WALLPAPER_DOMAIN", "wall.example.com")
	t.Setenv("WALLPAPER_HTTPS_ADDR", ":8443")
	t.Setenv("WALLPAPER_HTTP_ADDR", ":8080")

	cfg := Load()
	if cfg.HTTPSAddr != ":8443" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("addrs = %q / %q", cfg.HTTPSAddr, cfg.HTTPAddr)
	}
}

func TestSourceDefaultsToLocal(t *testing.T) {
	t.Setenv("WALLPAPER_SOURCE", "")
	t.Setenv("WALLPAPER_GITHUB_REPO", "")

	cfg := Load()
	if cfg.UsesGitHub() {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceLocal)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestRepositoryImpliesGitHubSource(t *testing.T) {
	t.Setenv("WALLPAPER_SOURCE", "")
	t.Setenv("WALLPAPER_GITHUB_REPO", "owner/name")

	cfg := Load()
	if !cfg.UsesGitHub() {
		t.Fatalf("Source = %q, want %q", cfg.Source, SourceGitHub)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if cfg.GitHubRef != DefaultGitHubRef || cfg.GitHubPath != DefaultGitHubPath {
		t.Fatalf("github defaults = %q / %q", cfg.GitHubRef, cfg.GitHubPath)
	}
	if cfg.GitHubRefreshInterval != DefaultGitHubRefresh {
		t.Fatalf("GitHubRefreshInterval = %s, want %s", cfg.GitHubRefreshInterval, DefaultGitHubRefresh)
	}
}

func TestExplicitGitHubSourceRequiresRepo(t *testing.T) {
	t.Setenv("WALLPAPER_SOURCE", SourceGitHub)
	t.Setenv("WALLPAPER_GITHUB_REPO", "")

	cfg := Load()
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want error when github source has no repository")
	}
}
