// Package catalog keeps the current manifest in memory and refreshes it from a
// local file or an HTTP endpoint.
package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"

	"wallpaper-api/internal/manifest"
)

// maxManifestBytes bounds how much of a remote manifest is read.
const maxManifestBytes = 16 << 20

// Fetcher returns the raw manifest document.
type Fetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// FileFetcher reads the manifest from a local file.
type FileFetcher struct {
	Path string
}

// Fetch reads the manifest file.
func (f FileFetcher) Fetch(context.Context) ([]byte, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", f.Path, err)
	}
	return data, nil
}

// HTTPFetcher downloads the manifest from a URL.
type HTTPFetcher struct {
	URL    string
	Token  string
	Client *http.Client
}

// Fetch downloads the manifest document.
func (f HTTPFetcher) Fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	if f.Token != "" {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch manifest: unexpected status %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes))
}

// Catalog caches the parsed manifest behind an atomic pointer so readers never
// block on a refresh.
type Catalog struct {
	fetcher  Fetcher
	snapshot atomic.Pointer[manifest.Manifest]
}

// New creates an empty catalog backed by fetcher.
func New(fetcher Fetcher) *Catalog { return &Catalog{fetcher: fetcher} }

// Refresh fetches, parses and swaps in a new manifest. The previous manifest is
// kept when the fetch or parse fails.
func (c *Catalog) Refresh(ctx context.Context) (int, error) {
	data, err := c.fetcher.Fetch(ctx)
	if err != nil {
		return 0, err
	}
	m, err := manifest.Parse(data)
	if err != nil {
		return 0, err
	}
	c.snapshot.Store(m)
	return m.Count, nil
}

// Snapshot returns the current manifest, or nil when none has been loaded.
func (c *Catalog) Snapshot() *manifest.Manifest {
	return c.snapshot.Load()
}
