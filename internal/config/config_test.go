package config

import "testing"

func TestParseHostListNormalizesAndDedupes(t *testing.T) {
	got := parseHostList(" Example.com , https://cdn.example.com/path , EXAMPLE.com ,, *.foo.com ")
	want := []string{"example.com", "cdn.example.com", "*.foo.com"}
	if len(got) != len(want) {
		t.Fatalf("parseHostList() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseHostList() = %v, want %v", got, want)
		}
	}
}

func TestParseHostListEmpty(t *testing.T) {
	for _, raw := range []string{"", "   ", ",", " , "} {
		if got := parseHostList(raw); got != nil {
			t.Fatalf("parseHostList(%q) = %v, want nil", raw, got)
		}
	}
}

func TestGitHubManifestURL(t *testing.T) {
	cfg := Config{
		Source:        SourceGitHub,
		GitHubRepo:    "owner/name",
		GitHubRef:     "main",
		GitHubRawBase: "https://raw.githubusercontent.com/",
	}
	want := "https://raw.githubusercontent.com/owner/name/main/manifest.json"
	if got := cfg.GitHubManifestURL(); got != want {
		t.Fatalf("GitHubManifestURL() = %q, want %q", got, want)
	}
	cfg.ManifestURL = "https://cdn.example.com/manifest.json"
	if got := cfg.GitHubManifestURL(); got != cfg.ManifestURL {
		t.Fatalf("explicit ManifestURL was ignored: %q", got)
	}
}

func TestValidate(t *testing.T) {
	if err := (Config{Source: SourceLocal}).Validate(); err != nil {
		t.Fatalf("local config rejected: %v", err)
	}
	if err := (Config{Source: SourceGitHub}).Validate(); err == nil {
		t.Fatal("github config without a repository was accepted")
	}
	if err := (Config{Source: SourceGitHub, GitHubRepo: "owner/name"}).Validate(); err != nil {
		t.Fatalf("valid github config rejected: %v", err)
	}
	if err := (Config{Source: "s3"}).Validate(); err == nil {
		t.Fatal("unknown source was accepted")
	}
}

func TestLoadInfersSource(t *testing.T) {
	t.Setenv("WALLPAPER_SOURCE", "")
	t.Setenv("WALLPAPER_GITHUB_REPO", "")
	if got := Load().Source; got != SourceLocal {
		t.Fatalf("source = %q, want %q", got, SourceLocal)
	}
	t.Setenv("WALLPAPER_GITHUB_REPO", "owner/name")
	if got := Load().Source; got != SourceGitHub {
		t.Fatalf("source = %q, want %q", got, SourceGitHub)
	}
}

func TestNormalizeHostKeepsWildcard(t *testing.T) {
	if got := normalizeHost("*.Example.com"); got != "*.example.com" {
		t.Fatalf("normalizeHost() = %q", got)
	}
}
