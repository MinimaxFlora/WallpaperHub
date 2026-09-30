// Package github lists wallpapers stored in a GitHub repository and proxies
// image bytes from GitHub so that clients only ever talk to this service.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"wallpaper-api/internal/index"
)

// defaultUserAgent is required by the GitHub API, which rejects requests
// without a User-Agent header.
const defaultUserAgent = "wallpaper-api"

// maxTreeBytes bounds how much of the tree response is read.
const maxTreeBytes = 8 << 20

// Config describes the repository to read wallpapers from.
type Config struct {
	APIBase   string // e.g. https://api.github.com
	RawBase   string // e.g. https://raw.githubusercontent.com
	Repo      string // e.g. owner/name
	Ref       string // branch, tag or commit
	Root      string // directory inside the repository holding the images
	Token     string // optional, raises the API rate limit
	UserAgent string
}

// Source reads the file list from the GitHub tree API and proxies image bytes
// from the raw content endpoint.
type Source struct {
	cfg          Config
	apiClient    *http.Client
	streamClient *http.Client
	logger       *slog.Logger

	snapshot atomic.Pointer[[]index.Image]
	allowed  atomic.Pointer[map[string]struct{}]
}

// treeEntry is a single item of the GitHub git tree response.
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

type treeResponse struct {
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// NewSource builds a GitHub-backed image source.
func NewSource(cfg Config, logger *slog.Logger) *Source {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	s := &Source{
		cfg: cfg,
		apiClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 20 * time.Second,
				MaxIdleConns:          32,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		// No overall timeout: image bodies are streamed to clients and may be
		// large, so the deadline is governed by per-phase timeouts instead.
		streamClient: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConns:          64,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		logger: logger,
	}
	empty := []index.Image{}
	s.snapshot.Store(&empty)
	emptySet := map[string]struct{}{}
	s.allowed.Store(&emptySet)
	return s
}

// Snapshot returns the current image list.
func (s *Source) Snapshot() []index.Image {
	if p := s.snapshot.Load(); p != nil {
		return *p
	}
	return nil
}

// Refresh fetches the repository tree and rebuilds the image index. On error
// the previous snapshot is kept.
func (s *Source) Refresh(ctx context.Context) (int, error) {
	tree, err := s.fetchTree(ctx)
	if err != nil {
		return 0, err
	}
	prefix := strings.Trim(s.cfg.Root, "/")
	images := make([]index.Image, 0, len(tree.Tree))
	allowed := make(map[string]struct{}, len(tree.Tree))
	for _, entry := range tree.Tree {
		if entry.Type != "blob" || !index.IsImageFile(entry.Path) {
			continue
		}
		rel, ok := relativePath(prefix, entry.Path)
		if !ok {
			continue
		}
		images = append(images, index.NewImage(rel, entry.Size, time.Time{}))
		allowed[rel] = struct{}{}
	}
	sort.Slice(images, func(i, j int) bool { return images[i].RelPath < images[j].RelPath })

	s.snapshot.Store(&images)
	s.allowed.Store(&allowed)
	if tree.Truncated {
		s.logger.Warn("github tree response was truncated; index may be incomplete",
			"repo", s.cfg.Repo, "ref", s.cfg.Ref)
	}
	return len(images), nil
}

// relativePath returns the path relative to root, or false when the entry is
// outside root.
func relativePath(root, p string) (string, bool) {
	if root == "" {
		return p, true
	}
	prefix := root + "/"
	if !strings.HasPrefix(p, prefix) {
		return "", false
	}
	return strings.TrimPrefix(p, prefix), true
}

func (s *Source) fetchTree(ctx context.Context) (*treeResponse, error) {
	endpoint := strings.TrimRight(s.cfg.APIBase, "/") +
		"/repos/" + s.cfg.Repo +
		"/git/trees/" + url.PathEscape(s.cfg.Ref) +
		"?recursive=1"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	if s.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	}

	resp, err := s.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github api %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out treeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxTreeBytes)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode github tree: %w", err)
	}
	return &out, nil
}

// upstreamURL builds the raw content URL for a relative image path.
func (s *Source) upstreamURL(relPath string) (string, error) {
	base, err := url.Parse(strings.TrimRight(s.cfg.RawBase, "/"))
	if err != nil {
		return "", err
	}
	base.Path = path.Join(base.Path, s.cfg.Repo, s.cfg.Ref, strings.Trim(s.cfg.Root, "/"), relPath)
	return base.String(), nil
}

// ServeImage proxies the image bytes from GitHub. Only paths present in the
// current index are served, so this endpoint cannot be used as an open proxy.
func (s *Source) ServeImage(w http.ResponseWriter, r *http.Request, relPath string) {
	if relPath == "" || strings.Contains(relPath, "..") {
		http.NotFound(w, r)
		return
	}
	if !s.isAllowed(relPath) {
		http.NotFound(w, r)
		return
	}

	upstream, err := s.upstreamURL(relPath)
	if err != nil {
		s.logger.Error("build upstream url", "path", relPath, "error", err)
		http.Error(w, "bad upstream", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	// Forward conditional and range headers for caching and resumable fetches.
	for _, h := range []string{"If-None-Match", "If-Modified-Since", "Range"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	if s.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.Token)
	}

	resp, err := s.streamClient.Do(req)
	if err != nil {
		s.logger.Error("fetch upstream image", "path", relPath, "error", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified", "Cache-Control"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}

	if resp.StatusCode == http.StatusNotModified {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		s.logger.Warn("upstream returned unexpected status",
			"path", relPath, "status", resp.StatusCode)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Debug("stream image body", "path", relPath, "error", err)
	}
}

func (s *Source) isAllowed(relPath string) bool {
	p := s.allowed.Load()
	if p == nil {
		return false
	}
	_, ok := (*p)[relPath]
	return ok
}
