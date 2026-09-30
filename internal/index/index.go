// Package index maintains an in-memory snapshot of the wallpapers available
// under a local root directory, and defines the Source contract shared with
// remote sources such as a GitHub repository.
package index

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// supportedExt lists the image extensions included in an index. Matching is
// case-insensitive.
var supportedExt = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"webp": true,
	"gif":  true,
	"bmp":  true,
	"avif": true,
}

// Image describes a single wallpaper file.
type Image struct {
	// RelPath is the slash-separated path relative to the source root. It is
	// used as the URL path segment after /images/.
	RelPath string
	// Name is the file name without its extension, used as the title.
	Name string
	// Size is the file size in bytes when known.
	Size int64
	// ModTime is the file modification time when known.
	ModTime time.Time
}

// Source provides the current wallpaper list and serves image bytes for a
// relative path. Implementations exist for a local directory and for a remote
// GitHub repository.
type Source interface {
	// Snapshot returns the current image list. The returned slice is immutable.
	Snapshot() []Image
	// ServeImage writes the image identified by relPath to the response.
	ServeImage(w http.ResponseWriter, r *http.Request, relPath string)
}

// IsImageFile reports whether name has a supported image extension.
func IsImageFile(name string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	return supportedExt[ext]
}

// NewImage builds an Image from a slash-separated relative path.
func NewImage(relPath string, size int64, modTime time.Time) Image {
	base := relPath
	if idx := strings.LastIndex(relPath, "/"); idx >= 0 {
		base = relPath[idx+1:]
	}
	return Image{
		RelPath: relPath,
		Name:    strings.TrimSuffix(base, filepath.Ext(base)),
		Size:    size,
		ModTime: modTime,
	}
}

// Sort sorts images in place by relative path for a stable order.
func Sort(images []Image) {
	sort.Slice(images, func(i, j int) bool {
		return images[i].RelPath < images[j].RelPath
	})
}

// Index is a concurrency-safe local directory index backed by an atomically
// replaced snapshot.
type Index struct {
	root     string
	snapshot atomic.Pointer[[]Image]
}

// New creates an empty index rooted at dir.
func New(dir string) *Index {
	ix := &Index{root: dir}
	empty := []Image{}
	ix.snapshot.Store(&empty)
	return ix
}

// Snapshot returns the current image list. The returned slice is immutable and
// must not be modified by callers.
func (ix *Index) Snapshot() []Image {
	if p := ix.snapshot.Load(); p != nil {
		return *p
	}
	return nil
}

// Len returns the number of indexed images.
func (ix *Index) Len() int {
	return len(ix.Snapshot())
}

// Refresh rescans the root directory. On error the previous snapshot is kept.
func (ix *Index) Refresh(_ context.Context) (int, error) {
	images, err := ix.scan()
	if err != nil {
		return 0, err
	}
	ix.snapshot.Store(&images)
	return len(images), nil
}

// ServeImage serves a file from the local root directory.
func (ix *Index) ServeImage(w http.ResponseWriter, r *http.Request, relPath string) {
	if relPath == "" || strings.Contains(relPath, "..") {
		http.NotFound(w, r)
		return
	}

	root, err := filepath.Abs(ix.root)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relPath)))
	if err != nil || (full != root && !strings.HasPrefix(full, root+string(os.PathSeparator))) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, full)
}

func (ix *Index) scan() ([]Image, error) {
	info, err := os.Stat(ix.root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("image root %q is not a directory", ix.root)
	}

	images := make([]Image, 0)
	walkErr := filepath.WalkDir(ix.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !IsImageFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(ix.root, path)
		if err != nil {
			return err
		}
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		images = append(images, NewImage(filepath.ToSlash(rel), fileInfo.Size(), fileInfo.ModTime()))
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	Sort(images)
	return images, nil
}
