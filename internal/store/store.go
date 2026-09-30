// Package store serves wallpaper bytes from a local directory or from a GitHub
// repository, caching remote bytes on disk.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotFound reports that the requested image does not exist.
var ErrNotFound = errors.New("image not found")

// Opened is an open image that is ready to be served. Close must be called
// once the caller is done with it.
type Opened struct {
	File     *os.File
	Size     int64
	ModTime  time.Time
	CacheHit bool
}

// Close closes the underlying file.
func (o Opened) Close() error {
	if o.File == nil {
		return nil
	}
	return o.File.Close()
}

// Store opens wallpaper bytes by their manifest-relative path.
type Store interface {
	Open(ctx context.Context, relPath string) (Opened, error)
}

// safeJoin joins a slash-separated relative path onto root, rejecting any path
// that would escape the root directory.
func safeJoin(root, rel string) (string, error) {
	cleaned := strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(rel, "\\", "/")), "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("empty image path")
	}
	return filepath.Join(root, filepath.FromSlash(cleaned)), nil
}

// openFile opens path and reports whether it came from a cache.
func openFile(path string, cacheHit bool) (Opened, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Opened{}, ErrNotFound
	}
	f, err := os.Open(path)
	if err != nil {
		return Opened{}, err
	}
	return Opened{File: f, Size: info.Size(), ModTime: info.ModTime(), CacheHit: cacheHit}, nil
}
