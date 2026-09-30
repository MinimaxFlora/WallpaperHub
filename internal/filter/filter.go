// Package filter normalizes the query parameters shared by /v1/random and
// /v1/images and matches them against manifest images.
package filter

import (
	"net/url"
	"strconv"
	"strings"

	"wallpaper-api/internal/errs"
	"wallpaper-api/internal/manifest"
)

// validOrientation lists the accepted orientation values.
var validOrientation = map[string]bool{
	"landscape": true,
	"portrait":  true,
	"square":    true,
}

// Criteria is a normalized filter set. Tags use AND semantics.
type Criteria struct {
	Tags        []string
	Category    string
	MinWidth    int
	MinHeight   int
	Orientation string
}

// Parse reads and validates the filter parameters. A malformed value yields an
// *errs.Error so the caller can render a structured 400 response.
func Parse(q url.Values) (Criteria, error) {
	var c Criteria
	c.Category = strings.TrimSpace(q.Get("category"))

	if raw := strings.TrimSpace(q.Get("tags")); raw != "" {
		seen := make(map[string]bool)
		for _, part := range strings.Split(raw, ",") {
			tag := strings.TrimSpace(part)
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			c.Tags = append(c.Tags, tag)
		}
	}

	var err error
	if c.MinWidth, err = parseSize(q.Get("min_width"), "min_width"); err != nil {
		return Criteria{}, err
	}
	if c.MinHeight, err = parseSize(q.Get("min_height"), "min_height"); err != nil {
		return Criteria{}, err
	}

	c.Orientation = strings.ToLower(strings.TrimSpace(q.Get("orientation")))
	if c.Orientation != "" && !validOrientation[c.Orientation] {
		return Criteria{}, errs.BadRequest("invalid_orientation", "orientation must be landscape, portrait or square")
	}
	return c, nil
}

func parseSize(raw, field string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, errs.BadRequest("invalid_"+field, field+" must be a non-negative integer")
	}
	return v, nil
}

// Match reports whether img satisfies every criterion.
func (c Criteria) Match(img manifest.Image) bool {
	if c.Category != "" && img.Category != c.Category {
		return false
	}
	if c.MinWidth > 0 && img.Width < c.MinWidth {
		return false
	}
	if c.MinHeight > 0 && img.Height < c.MinHeight {
		return false
	}
	if c.Orientation != "" && img.Orientation != c.Orientation {
		return false
	}
	if len(c.Tags) > 0 {
		have := make(map[string]bool, len(img.Tags))
		for _, t := range img.Tags {
			have[t] = true
		}
		for _, want := range c.Tags {
			if !have[want] {
				return false
			}
		}
	}
	return true
}

// Apply returns the images satisfying the criteria, preserving input order.
func Apply(images []manifest.Image, c Criteria) []manifest.Image {
	out := make([]manifest.Image, 0, len(images))
	for _, img := range images {
		if c.Match(img) {
			out = append(out, img)
		}
	}
	return out
}
