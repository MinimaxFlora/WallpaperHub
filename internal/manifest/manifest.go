// Package manifest defines the wallpaper catalogue model shared by the
// manifest generator and the HTTP API.
package manifest

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

// supportedExt maps a lower-case file extension to its canonical format name.
var supportedExt = map[string]string{
	"jpg":  "jpeg",
	"jpeg": "jpeg",
	"png":  "png",
	"webp": "webp",
	"avif": "avif",
	"gif":  "gif",
}

// Image describes one wallpaper.
type Image struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Orientation string   `json:"orientation"`
	Format      string   `json:"format"`
	Bytes       int64    `json:"bytes"`
	Hash        string   `json:"hash"`
}

// Manifest is the JSON document listing every wallpaper. Paths are relative to
// the repository root, for example "images/01.webp".
type Manifest struct {
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Timezone    string    `json:"timezone"`
	Count       int       `json:"count"`
	Images      []Image   `json:"images"`
}

// Format returns the canonical format name for a file name, or "" when the
// extension is not a supported image type.
func Format(name string) string {
	return supportedExt[strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))]
}

// IsImage reports whether name has a supported image extension.
func IsImage(name string) bool { return Format(name) != "" }

// Orientation derives landscape, portrait or square from the dimensions.
func Orientation(width, height int) string {
	switch {
	case width > height:
		return "landscape"
	case width < height:
		return "portrait"
	default:
		return "square"
	}
}

// Parse decodes and validates a manifest document.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if m.Version <= 0 {
		return nil, fmt.Errorf("manifest version must be positive, got %d", m.Version)
	}
	seen := make(map[string]struct{}, len(m.Images))
	for i := range m.Images {
		img := &m.Images[i]
		if img.ID == "" {
			return nil, fmt.Errorf("manifest image %d has an empty id", i)
		}
		if _, dup := seen[img.ID]; dup {
			return nil, fmt.Errorf("duplicate image id %q", img.ID)
		}
		seen[img.ID] = struct{}{}
		if img.Path == "" {
			return nil, fmt.Errorf("image %q has an empty path", img.ID)
		}
		if img.Hash == "" {
			return nil, fmt.Errorf("image %q has an empty hash", img.ID)
		}
		if img.Orientation == "" {
			img.Orientation = Orientation(img.Width, img.Height)
		}
		if img.Tags == nil {
			img.Tags = []string{}
		}
	}
	sort.Slice(m.Images, func(i, j int) bool { return m.Images[i].ID < m.Images[j].ID })
	m.Count = len(m.Images)
	return &m, nil
}

// Find returns the image with the given id.
func (m *Manifest) Find(id string) (Image, bool) {
	for _, img := range m.Images {
		if img.ID == id {
			return img, true
		}
	}
	return Image{}, false
}
