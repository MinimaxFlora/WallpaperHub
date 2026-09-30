// Package index maintains an in-memory snapshot of the wallpapers available
// under a root directory.
package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// supportedExt lists the image extensions included in the index. Matching is
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
	// RelPath is the slash-separated path relative to the index root. It is
	// used as the URL path segment after /images/.
	RelPath string
	// Name is the file name without its extension, used as the title.
	Name string
	// ModTime is the file modification time.
	ModTime time.Time
}

// Index is a concurrency-safe collection of images backed by an atomically
// replaced snapshot.
type Index struct {
	root      string
	snapshot  atomic.Pointer[[]Image]
	lastCount atomic.Int64
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
func (ix *Index) Refresh() (int, error) {
	images, err := scan(ix.root)
	if err != nil {
		return 0, err
	}
	ix.snapshot.Store(&images)
	ix.lastCount.Store(int64(len(images)))
	return len(images), nil
}

// LastCount returns the number of images captured by the most recent
// successful scan.
func (ix *Index) LastCount() int {
	return int(ix.lastCount.Load())
}

func scan(root string) ([]Image, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("image root %q is not a directory", root)
	}

	images := make([]Image, 0)
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), "."))
		if !supportedExt[ext] {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		images = append(images, Image{
			RelPath: filepath.ToSlash(rel),
			Name:    strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
			ModTime: fileInfo.ModTime(),
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(images, func(i, j int) bool {
		return images[i].RelPath < images[j].RelPath
	})
	return images, nil
}
