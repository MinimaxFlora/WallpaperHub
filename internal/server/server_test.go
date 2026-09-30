package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wallpaper-api/internal/catalog"
	"wallpaper-api/internal/config"
	"wallpaper-api/internal/store"
)

const testManifest = `{
  "version": 1,
  "generated_at": "2026-09-30T00:00:00Z",
  "timezone": "Asia/Shanghai",
  "count": 2,
  "images": [
    {"id":"01","path":"images/01.webp","title":"One","category":"anime","tags":["anime","blue"],"width":1920,"height":1080,"orientation":"landscape","format":"webp","bytes":8,"hash":"sha256-aaaa"},
    {"id":"02","path":"images/02.webp","title":"Two","category":"landscape","tags":[],"width":1080,"height":1920,"orientation":"portrait","format":"webp","bytes":8,"hash":"sha256-bbbb"}
  ]
}`

type staticFetcher struct{ data string }

func (f staticFetcher) Fetch(context.Context) ([]byte, error) { return []byte(f.data), nil }

func baseConfig() config.Config {
	return config.Config{Location: time.UTC, Timezone: "UTC"}
}

// newTestHandler builds a handler over a temporary image directory. An empty
// document leaves the catalogue empty so the unavailable path can be tested.
func newTestHandler(t *testing.T, cfg config.Config, document string) http.Handler {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"01.webp": "01234567", "02.webp": "abcdefgh"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cat := catalog.New(staticFetcher{data: document})
	if document != "" {
		if _, err := cat.Refresh(context.Background()); err != nil {
			t.Fatalf("catalog refresh: %v", err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(cfg, cat, store.NewLocal(root), logger).Handler()
}

func do(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeView(t *testing.T, rec *httptest.ResponseRecorder) imageView {
	t.Helper()
	var v imageView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode image: %v (body %s)", err, rec.Body.String())
	}
	return v
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) listResponse {
	t.Helper()
	var v listResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode list: %v (body %s)", err, rec.Body.String())
	}
	return v
}

func decodeCount(t *testing.T, rec *httptest.ResponseRecorder) countResponse {
	t.Helper()
	var v countResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode counts: %v (body %s)", err, rec.Body.String())
	}
	return v
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error: %v (body %s)", err, rec.Body.String())
	}
	return resp.Error.Code
}

func TestRandomSeedIsStable(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	first := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=seed&seed=abc", nil))
	for i := 0; i < 5; i++ {
		got := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=seed&seed=abc", nil))
		if got.ID != first.ID {
			t.Fatalf("seed mode returned %q then %q", first.ID, got.ID)
		}
	}
	if first.Mode != "seed" {
		t.Fatalf("mode = %q, want seed", first.Mode)
	}
	if first.ManifestVersion != 1 {
		t.Fatalf("manifest_version = %d, want 1", first.ManifestVersion)
	}
	if first.URL == "" || first.PageURL == "" {
		t.Fatal("image URLs were not populated")
	}
}

func TestRandomDefaultModePicksFromCatalogue(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	got := decodeView(t, do(h, http.MethodGet, "/v1/random", nil))
	if got.ID != "01" && got.ID != "02" {
		t.Fatalf("unexpected id %q", got.ID)
	}
	if got.Mode != "random" {
		t.Fatalf("mode = %q, want random", got.Mode)
	}
}

func TestRandomUnknownModeFallsBackToRandom(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	if got := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=bogus", nil)); got.Mode != "random" {
		t.Fatalf("mode = %q, want random", got.Mode)
	}
}

func TestRandomDailyDate(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	first := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=daily&date=2024-05-05", nil))
	second := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=daily&date=2024-05-05", nil))
	if first.ID != second.ID {
		t.Fatalf("daily mode is not stable: %q then %q", first.ID, second.ID)
	}

	rec := do(h, http.MethodGet, "/v1/random?mode=daily&date=05-05-2024", nil)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_date" {
		t.Fatalf("invalid date: status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestRandomSessionIsStableWithExplicitId(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	first := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=session&session=client-1", nil))
	second := decodeView(t, do(h, http.MethodGet, "/v1/random?mode=session&session=client-1", nil))
	if first.ID != second.ID {
		t.Fatalf("session mode is not stable: %q then %q", first.ID, second.ID)
	}
}

func TestRandomSessionSetsCookieWhenMissing(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/random?mode=session", nil)
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "wallpaper_session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("session mode did not set a session cookie")
	}
}

func TestRandomNoMatch(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/random?category=missing", nil)
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "no_match" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestRandomRedirect(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/random?redirect=1", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	location := rec.Header().Get("Location")
	if location == "" || location[len(location)-5:] != "/file" {
		t.Fatalf("unexpected location %q", location)
	}
}

func TestRandomInvalidOrientation(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/random?orientation=diagonal", nil)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_orientation" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestListPagination(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)

	all := decodeList(t, do(h, http.MethodGet, "/v1/images", nil))
	if all.Total != 2 || all.Page != 1 || all.PageSize != defaultPageSize || len(all.Items) != 2 {
		t.Fatalf("default listing = %+v", all)
	}

	page2 := decodeList(t, do(h, http.MethodGet, "/v1/images?page_size=1&page=2", nil))
	if page2.Total != 2 || page2.Page != 2 || len(page2.Items) != 1 || page2.Items[0].ID != "02" {
		t.Fatalf("page 2 = %+v", page2)
	}
}

func TestListRejectsOversizedPageSize(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/images?page_size=1000", nil)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "invalid_page_size" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestImageMeta(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	got := decodeView(t, do(h, http.MethodGet, "/v1/images/01", nil))
	if got.ID != "01" || got.Title != "One" || got.Category != "anime" {
		t.Fatalf("meta = %+v", got)
	}
	if got.Mode != "" {
		t.Fatalf("metadata response leaked mode %q", got.Mode)
	}

	rec := do(h, http.MethodGet, "/v1/images/99", nil)
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestImageFileServesBytes(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/images/01/file", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "01234567" {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if got := rec.Header().Get("ETag"); got != `"sha256-aaaa"` {
		t.Fatalf("etag = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("content type = %q", got)
	}
	if got := rec.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("cache = %q", got)
	}
}

func TestImageFileRange(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/images/01/file", map[string]string{"Range": "bytes=0-3"})
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if rec.Body.String() != "0123" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestImageFileConditional(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/images/01/file", map[string]string{"If-None-Match": `"sha256-aaaa"`})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
}

func TestImageFileUnknownId(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/v1/images/99/file", nil)
	if rec.Code != http.StatusNotFound || errorCode(t, rec) != "not_found" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
}

func TestHotlinkAllowlist(t *testing.T) {
	cfg := baseConfig()
	cfg.HotlinkAllowlist = []string{"example.com", "*.trusted.com"}
	h := newTestHandler(t, cfg, testManifest)

	cases := []struct {
		name   string
		origin string
		want   int
	}{
		{"exact", "https://example.com", http.StatusOK},
		{"wildcard", "https://img.trusted.com", http.StatusOK},
		{"apex of wildcard", "https://trusted.com", http.StatusOK},
		{"blocked", "https://evil.com", http.StatusForbidden},
		{"no origin", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			headers := map[string]string{}
			if tc.origin != "" {
				headers["Origin"] = tc.origin
			}
			rec := do(h, http.MethodGet, "/v1/images/01/file", headers)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && errorCode(t, rec) != "forbidden_origin" {
				t.Fatalf("code = %s", errorCode(t, rec))
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	cfg := baseConfig()
	cfg.RateLimitLimit = 1
	cfg.RateLimitWindow = time.Minute
	h := newTestHandler(t, cfg, testManifest)

	if rec := do(h, http.MethodGet, "/healthz", nil); rec.Code != http.StatusOK {
		t.Fatalf("first request status = %d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("429 response is missing Retry-After")
	}
	if errorCode(t, rec) != "rate_limited" {
		t.Fatalf("code = %s", errorCode(t, rec))
	}
}

func TestTagsAndCategories(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)

	tags := decodeCount(t, do(h, http.MethodGet, "/v1/tags", nil))
	if !hasCount(tags, "anime", 1) || !hasCount(tags, "blue", 1) {
		t.Fatalf("tags = %+v", tags.Items)
	}
	cats := decodeCount(t, do(h, http.MethodGet, "/v1/categories", nil))
	if !hasCount(cats, "anime", 1) || !hasCount(cats, "landscape", 1) {
		t.Fatalf("categories = %+v", cats.Items)
	}
}

func TestHealth(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if resp.Status != "ok" || resp.Images != 2 || resp.ManifestGeneratedAt == "" {
		t.Fatalf("health = %+v", resp)
	}
}

func TestManifestUnavailable(t *testing.T) {
	h := newTestHandler(t, baseConfig(), "")
	rec := do(h, http.MethodGet, "/v1/random", nil)
	if rec.Code != http.StatusServiceUnavailable || errorCode(t, rec) != "manifest_unavailable" {
		t.Fatalf("status %d, code %s", rec.Code, errorCode(t, rec))
	}
	if hrec := do(h, http.MethodGet, "/healthz", nil); hrec.Code != http.StatusServiceUnavailable {
		t.Fatalf("health status = %d, want 503", hrec.Code)
	}
}

func TestMethodNotAllowedAndPreflight(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	if rec := do(h, http.MethodPost, "/v1/random", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
	rec := do(h, http.MethodOptions, "/v1/random", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS origin = %q", got)
	}
}

func TestIndexAndUnknownRoute(t *testing.T) {
	h := newTestHandler(t, baseConfig(), testManifest)
	rec := do(h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d", rec.Code)
	}
	var resp indexResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode index: %v", err)
	}
	if resp.Service != "wallpaper-api" || resp.Images != 2 || len(resp.Endpoints) == 0 {
		t.Fatalf("index = %+v", resp)
	}
	if rec := do(h, http.MethodGet, "/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", rec.Code)
	}
}

func hasCount(resp countResponse, name string, count int) bool {
	for _, item := range resp.Items {
		if item.Name == name && item.Count == count {
			return true
		}
	}
	return false
}
