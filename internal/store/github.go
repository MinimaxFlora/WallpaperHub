package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// maxImageBytes bounds how much of a single image is downloaded.
const maxImageBytes = 64 << 20

// GitHubStore fetches images from raw GitHub content and caches them on disk so
// that repeated requests avoid the network.
type GitHubStore struct {
	rawBase   string
	repo      string
	ref       string
	token     string
	userAgent string
	cacheDir  string
	client    *http.Client
	logger    *slog.Logger
}

// GitHubConfig configures a GitHubStore.
type GitHubConfig struct {
	RawBase   string
	Repo      string
	Ref       string
	Token     string
	UserAgent string
	CacheDir  string
	Client    *http.Client
	Logger    *slog.Logger
}

// NewGitHub builds a GitHub-backed store. The cache directory is created when
// missing.
func NewGitHub(cfg GitHubConfig) (*GitHubStore, error) {
	if cfg.UserAgent == "" {
		cfg.UserAgent = "wallpaper-api"
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConns:          32,
				IdleConnTimeout:       90 * time.Second,
			},
		}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &GitHubStore{
		rawBase:   cfg.RawBase,
		repo:      cfg.Repo,
		ref:       cfg.Ref,
		token:     cfg.Token,
		userAgent: cfg.UserAgent,
		cacheDir:  cfg.CacheDir,
		client:    cfg.Client,
		logger:    logger,
	}, nil
}

// Open returns the cached file when present, otherwise downloads it first.
func (s *GitHubStore) Open(ctx context.Context, relPath string) (Opened, error) {
	cachePath, err := safeJoin(s.cacheDir, relPath)
	if err != nil {
		return Opened{}, ErrNotFound
	}
	if _, err := os.Stat(cachePath); err == nil {
		return openFile(cachePath, true)
	}
	if err := s.download(ctx, relPath, cachePath); err != nil {
		return Opened{}, err
	}
	return openFile(cachePath, false)
}

// download streams the raw image into the cache directory atomically.
func (s *GitHubStore) download(ctx context.Context, relPath, dest string) error {
	endpoint, err := s.rawURL(relPath)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", s.userAgent)
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", relPath, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("fetch %s: unexpected status %s", relPath, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, io.LimitReader(resp.Body, maxImageBytes)); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// rawURL builds the raw content URL for a manifest-relative image path.
func (s *GitHubStore) rawURL(relPath string) (string, error) {
	base, err := url.Parse(strings.TrimRight(s.rawBase, "/"))
	if err != nil {
		return "", err
	}
	base.Path = path.Join(base.Path, s.repo, s.ref, relPath)
	return base.String(), nil
}
