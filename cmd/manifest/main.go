// Command manifest regenerates manifest.json from the images directory and the
// metadata file. It is intended to run in CI after images change, committing the
// refreshed manifest back to the repository.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wallpaper-api/internal/imaging"
	"wallpaper-api/internal/manifest"
)

// metadataEntry is one record of metadata.json, keyed by repository path.
type metadataEntry struct {
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	images := flag.String("images", "images", "image directory relative to root")
	metadataPath := flag.String("metadata", "metadata.json", "metadata file relative to root")
	out := flag.String("out", "manifest.json", "output manifest path relative to root")
	timezone := flag.String("timezone", "Asia/Shanghai", "timezone recorded in the manifest")
	version := flag.Int("version", 1, "manifest schema version")
	flag.Parse()

	if err := run(*root, *images, *metadataPath, *out, *timezone, *version); err != nil {
		log.Fatalf("manifest: %v", err)
	}
}

func run(root, imagesDir, metadataPath, outPath, timezone string, version int) error {
	if _, err := time.LoadLocation(timezone); err != nil {
		return fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}
	entries, err := loadMetadata(filepath.Join(root, metadataPath))
	if err != nil {
		return err
	}

	images, err := collect(root, imagesDir, entries)
	if err != nil {
		return err
	}

	next := manifest.Manifest{
		Version:  version,
		Timezone: timezone,
		Count:    len(images),
		Images:   images,
	}
	out := filepath.Join(root, outPath)

	// Preserve the previous generation timestamp when nothing else changed, so
	// repeated runs stay idempotent and do not produce empty commits.
	if prev, err := readManifest(out); err == nil && sameImages(prev.Images, next.Images) {
		log.Printf("manifest unchanged (%d images); leaving %s untouched", len(images), out)
		return nil
	}
	next.GeneratedAt = time.Now().UTC().Truncate(time.Second)
	if err := writeManifest(out, next); err != nil {
		return err
	}
	log.Printf("wrote %s with %d images", out, len(images))
	return nil
}

// loadMetadata reads metadata.json. A missing file is treated as empty.
func loadMetadata(path string) (map[string]metadataEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]metadataEntry{}, nil
		}
		return nil, err
	}
	var raw map[string]metadataEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if raw == nil {
		raw = map[string]metadataEntry{}
	}
	return raw, nil
}

// collect walks the image directory and builds a sorted, validated image list.
func collect(root, imagesDir string, entries map[string]metadataEntry) ([]manifest.Image, error) {
	dir := filepath.Join(root, imagesDir)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("image directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	imagesRel := filepath.ToSlash(imagesDir)

	images := make([]manifest.Image, 0)
	seen := make(map[string]string)
	walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") || !manifest.IsImage(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		img, err := describe(rel, p, imagesRel, entries[rel])
		if err != nil {
			return err
		}
		if other, dup := seen[img.ID]; dup {
			return fmt.Errorf("duplicate image id %q (from %s and %s)", img.ID, other, rel)
		}
		seen[img.ID] = rel
		images = append(images, img)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(images, func(i, j int) bool { return images[i].ID < images[j].ID })
	return images, nil
}

// describe computes the metadata of a single image file.
func describe(rel, fullPath, imagesRoot string, meta metadataEntry) (manifest.Image, error) {
	id := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	format := manifest.Format(rel)

	f, err := os.Open(fullPath)
	if err != nil {
		return manifest.Image{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return manifest.Image{}, err
	}

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return manifest.Image{}, fmt.Errorf("hash %s: %w", rel, err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return manifest.Image{}, err
	}

	width, height := 0, 0
	if w, h, err := imaging.Dimensions(f, format); err != nil {
		log.Printf("warning: %s: cannot determine dimensions: %v", rel, err)
	} else {
		width, height = w, h
	}

	title := strings.TrimSpace(meta.Title)
	if title == "" {
		title = id
	}
	category := strings.TrimSpace(meta.Category)
	if category == "" {
		category = defaultCategory(rel, imagesRoot)
	}
	tags := meta.Tags
	if tags == nil {
		tags = []string{}
	}

	return manifest.Image{
		ID:          id,
		Path:        rel,
		Title:       title,
		Category:    category,
		Tags:        tags,
		Width:       width,
		Height:      height,
		Orientation: manifest.Orientation(width, height),
		Format:      format,
		Bytes:       info.Size(),
		Hash:        "sha256-" + hex.EncodeToString(sum.Sum(nil)),
	}, nil
}

// defaultCategory falls back to the containing directory name, or
// "uncategorized" for images sitting directly in the image root.
func defaultCategory(rel, imagesRoot string) string {
	dir := path.Dir(rel)
	imagesRoot = strings.TrimSuffix(imagesRoot, "/")
	if imagesRoot == "." || imagesRoot == "" || dir == imagesRoot || dir == "." {
		return "uncategorized"
	}
	return path.Base(dir)
}

func readManifest(path string) (*manifest.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return manifest.Parse(data)
}

func sameImages(a, b []manifest.Image) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func writeManifest(path string, m manifest.Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
