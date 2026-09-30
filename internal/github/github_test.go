package github

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTreeServer returns an httptest server acting as the GitHub tree API.
func newTreeServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/owner/name/git/trees/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRefreshBuildsFilteredSortedIndex(t *testing.T) {
	tree := `{"tree":[
		{"path":"images/","type":"tree"},
		{"path":"images/b-second.png","type":"blob","size":20},
		{"path":"images/a-first.jpg","type":"blob","size":10},
		{"path":"images/readme.md","type":"blob","size":5},
		{"path":"docs/other.png","type":"blob","size":7},
		{"path":"images/nested/c-third.webp","type":"blob","size":30}
	]}`
	api := newTreeServer(t, tree)

	src := NewSource(Config{
		APIBase:   api.URL,
		RawBase:   "http://raw.example",
		Repo:      "owner/name",
		Ref:       "master",
		Root:      "images",
		UserAgent: "test",
	}, discardLogger())

	count, err := src.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("Refresh() count = %d, want 3", count)
	}

	got := src.Snapshot()
	want := []string{"a-first.jpg", "b-second.png", "nested/c-third.webp"}
	if len(got) != len(want) {
		t.Fatalf("Snapshot() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i].RelPath != name {
			t.Fatalf("Snapshot()[%d].RelPath = %q, want %q", i, got[i].RelPath, name)
		}
	}
	if got[0].Name != "a-first" {
		t.Fatalf("Name = %q, want %q", got[0].Name, "a-first")
	}
	if got[0].Size != 10 {
		t.Fatalf("Size = %d, want 10", got[0].Size)
	}
}

func TestRefreshErrorKeepsPreviousSnapshot(t *testing.T) {
	tree := `{"tree":[{"path":"images/keep.png","type":"blob","size":1}]}`
	api := newTreeServer(t, tree)

	src := NewSource(Config{
		APIBase: api.URL, RawBase: "http://raw.example",
		Repo: "owner/name", Ref: "master", Root: "images", UserAgent: "test",
	}, discardLogger())
	if _, err := src.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	src.cfg.APIBase = "http://127.0.0.1:1"
	if _, err := src.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() error = nil, want failure")
	}
	if got := src.Snapshot(); len(got) != 1 || got[0].RelPath != "keep.png" {
		t.Fatalf("Snapshot() after failed refresh = %v, want previous index", got)
	}
}

func TestServeImageProxiesAllowedPath(t *testing.T) {
	body := "PNG-BYTES"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/owner/name/master/images/pic.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(raw.Close)

	tree := `{"tree":[{"path":"images/pic.png","type":"blob","size":9}]}`
	api := newTreeServer(t, tree)

	src := NewSource(Config{
		APIBase: api.URL, RawBase: raw.URL,
		Repo: "owner/name", Ref: "master", Root: "images", UserAgent: "test",
	}, discardLogger())
	if _, err := src.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/images/pic.png", nil)
	src.ServeImage(rec, req, "pic.png")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != body {
		t.Fatalf("body = %q, want %q", rec.Body.String(), body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q, want upstream value", cc)
	}

	// A path that exists upstream but is absent from the index must not be
	// proxied, so the endpoint cannot be used as an open proxy.
	rec = httptest.NewRecorder()
	src.ServeImage(rec, httptest.NewRequest(http.MethodGet, "/images/secret.png", nil), "secret.png")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d, want 404", rec.Code)
	}
}

func TestServeImageDefaultsCacheControl(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		_, _ = io.WriteString(w, "x")
	}))
	t.Cleanup(raw.Close)

	tree := `{"tree":[{"path":"images/pic.webp","type":"blob","size":1}]}`
	src := NewSource(Config{
		APIBase: newTreeServer(t, tree).URL, RawBase: raw.URL,
		Repo: "owner/name", Ref: "master", Root: "images", UserAgent: "test",
	}, discardLogger())
	if _, err := src.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	rec := httptest.NewRecorder()
	src.ServeImage(rec, httptest.NewRequest(http.MethodGet, "/images/pic.webp", nil), "pic.webp")
	if cc := rec.Header().Get("Cache-Control"); cc == "" {
		t.Fatal("Cache-Control = empty, want default")
	}
}

func TestRelativePath(t *testing.T) {
	cases := []struct {
		root, in, want string
		ok             bool
	}{
		{"images", "images/a.png", "a.png", true},
		{"images", "images/nested/a.png", "nested/a.png", true},
		{"images", "other/a.png", "", false},
		{"", "a.png", "a.png", true},
		{"images", "images", "", false},
	}
	for _, tc := range cases {
		got, ok := relativePath(tc.root, tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("relativePath(%q, %q) = (%q, %v), want (%q, %v)", tc.root, tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
