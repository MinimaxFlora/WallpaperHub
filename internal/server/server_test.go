package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wallpaper-api/internal/config"
	"wallpaper-api/internal/index"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func newTestServer(t *testing.T, dir string, mutate func(*config.Config)) *Server {
	t.Helper()
	cfg := config.Config{
		ImagesDir:      dir,
		Copyright:      "Test Collection",
		Location:       time.UTC,
		RescanInterval: 0,
		LogLevel:       slog.LevelError,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	idx := index.New(dir)
	if _, err := idx.Refresh(); err != nil {
		t.Fatalf("index refresh: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, idx, logger)
}

func do(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = "wall.example.com"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t, t.TempDir(), nil)
	rec := do(t, s.Handler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestArchiveEmptyIndex(t *testing.T) {
	s := newTestServer(t, t.TempDir(), nil)
	rec := do(t, s.Handler(), "/HPImageArchive.aspx?format=js&idx=0&n=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"images": []`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestArchiveReturnsRequestedCountAndURLs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg"} {
		writeFile(t, filepath.Join(dir, name), name)
	}
	s := newTestServer(t, dir, nil)
	s.now = func() time.Time { return time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC) }

	rec := do(t, s.Handler(), "/HPImageArchive.aspx?format=js&idx=0&n=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if got := strings.Count(body, `"url":`); got != 3 {
		t.Fatalf("url count = %d, want 3\n%s", got, body)
	}
	if !strings.Contains(body, `"url": "http://wall.example.com/images/`) {
		t.Fatalf("url not absolute:\n%s", body)
	}
	if !strings.Contains(body, `"startdate": "20260930"`) {
		t.Fatalf("startdate missing:\n%s", body)
	}
	if cors := rec.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Fatalf("CORS header = %q", cors)
	}
}

func TestArchiveCountClampsToTotal(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg"} {
		writeFile(t, filepath.Join(dir, name), name)
	}
	s := newTestServer(t, dir, nil)
	rec := do(t, s.Handler(), "/HPImageArchive.aspx?n=50")
	if got := strings.Count(rec.Body.String(), `"url":`); got != 2 {
		t.Fatalf("url count = %d, want 2", got)
	}
}

func TestArchiveInvalidParamsFallBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"), "a")
	s := newTestServer(t, dir, nil)
	s.now = func() time.Time { return time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) }

	for _, target := range []string{
		"/HPImageArchive.aspx?idx=abc&n=xyz",
		"/HPImageArchive.aspx?idx=-4&n=0",
	} {
		rec := do(t, s.Handler(), target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, rec.Code)
		}
		if got := strings.Count(rec.Body.String(), `"url":`); got != 1 {
			t.Fatalf("%s url count = %d, want 1", target, got)
		}
		if !strings.Contains(rec.Body.String(), `"startdate": "20260105"`) {
			t.Fatalf("%s default date wrong:\n%s", target, rec.Body.String())
		}
	}
}

func TestArchiveIdxShiftsDateBackwards(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"), "a")
	s := newTestServer(t, dir, nil)
	s.now = func() time.Time { return time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC) }

	rec := do(t, s.Handler(), "/HPImageArchive.aspx?idx=1")
	if !strings.Contains(rec.Body.String(), `"startdate": "20260929"`) {
		t.Fatalf("idx=1 should shift to the previous day:\n%s", rec.Body.String())
	}
}

func TestArchiveIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		writeFile(t, filepath.Join(dir, name), name)
	}
	s := newTestServer(t, dir, nil)
	s.now = func() time.Time { return time.Date(2026, 9, 30, 6, 0, 0, 0, time.UTC) }

	first := do(t, s.Handler(), "/HPImageArchive.aspx?idx=0&n=2").Body.String()
	second := do(t, s.Handler(), "/HPImageArchive.aspx?idx=0&n=2").Body.String()
	if first != second {
		t.Fatalf("response not deterministic:\n%s\n---\n%s", first, second)
	}
}

func TestArchiveUsesConfiguredBaseURL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"), "a")
	s := newTestServer(t, dir, func(c *config.Config) {
		c.BaseURL = "https://cdn.example.com/wall"
	})
	rec := do(t, s.Handler(), "/HPImageArchive.aspx")
	if !strings.Contains(rec.Body.String(), `"url": "https://cdn.example.com/wall/images/a.jpg"`) {
		t.Fatalf("configured base url not used:\n%s", rec.Body.String())
	}
}

func TestArchiveHonorsForwardedProto(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.jpg"), "a")
	s := newTestServer(t, dir, nil)

	req := httptest.NewRequest(http.MethodGet, "/HPImageArchive.aspx", nil)
	req.Host = "wall.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"url": "https://wall.example.com/images/a.jpg"`) {
		t.Fatalf("forwarded proto ignored:\n%s", rec.Body.String())
	}
}

func TestImageServing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "nested", "pic.jpg"), "JPEGDATA")
	s := newTestServer(t, dir, nil)

	rec := do(t, s.Handler(), "/images/nested/pic.jpg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "JPEGDATA" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if cors := rec.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Fatalf("CORS header = %q", cors)
	}
}

func TestImageServingMissingAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "nested", "pic.jpg"), "x")
	s := newTestServer(t, dir, nil)

	for _, target := range []string{"/images/missing.jpg", "/images/nested/", "/images/"} {
		rec := do(t, s.Handler(), target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, rec.Code)
		}
	}
}

func TestImageServingBlocksTraversal(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "images")
	writeFile(t, filepath.Join(dir, "ok.jpg"), "ok")
	writeFile(t, filepath.Join(parent, "secret.txt"), "top-secret")
	s := newTestServer(t, dir, nil)

	req := httptest.NewRequest(http.MethodGet, "/images/x", nil)
	req.URL.Path = "/images/../secret.txt"
	rec := httptest.NewRecorder()
	s.handleImage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d, want 404", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "top-secret") {
		t.Fatal("traversal leaked file contents")
	}
}

func TestImageServingConditionalNotModified(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pic.jpg"), "data")
	s := newTestServer(t, dir, nil)

	first := do(t, s.Handler(), "/images/pic.jpg")
	lastMod := first.Header().Get("Last-Modified")
	if lastMod == "" {
		t.Fatal("missing Last-Modified header")
	}

	req := httptest.NewRequest(http.MethodGet, "/images/pic.jpg", nil)
	req.Header.Set("If-Modified-Since", lastMod)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
}

func TestOptionsPreflight(t *testing.T) {
	s := newTestServer(t, t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodOptions, "/HPImageArchive.aspx", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if cors := rec.Header().Get("Access-Control-Allow-Origin"); cors != "*" {
		t.Fatalf("CORS header = %q", cors)
	}
}
