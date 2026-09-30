// Package bing renders the Bing HPImageArchive-compatible response payload.
package bing

import (
	"crypto/sha1"
	"encoding/hex"
	"path"
	"strings"
	"time"
)

const dateLayout = "20060102"

// Image mirrors a single element of the Bing images array. Field names follow
// the official Bing HPImageArchive response.
type Image struct {
	URL           string `json:"url"`
	URLBase       string `json:"urlbase"`
	Copyright     string `json:"copyright"`
	CopyrightLink string `json:"copyrightlink"`
	Title         string `json:"title"`
	StartDate     string `json:"startdate"`
	FullStartDate string `json:"fullstartdate"`
	EndDate       string `json:"enddate"`
	WP            bool   `json:"wp"`
	HSH           string `json:"hsh"`
	Drk           int    `json:"drk"`
	Top           int    `json:"top"`
	Bot           int    `json:"bot"`
	Quiz          string `json:"quiz"`
}

// Response is the top-level payload returned by the API.
type Response struct {
	Images []Image `json:"images"`
}

// Renderer builds Bing-compatible image entries from indexed files.
type Renderer struct {
	// Copyright is the collection label appended to every copyright string.
	Copyright string
}

// URLPath returns the absolute URL path that serves the given relative image path.
func URLPath(relPath string) string {
	return "/images/" + strings.TrimPrefix(path.Clean("/"+relPath), "/")
}

// Render builds one image entry. baseURL must not have a trailing slash.
func (r Renderer) Render(baseURL, relPath, title string, date time.Time) Image {
	imageURL := baseURL + URLPath(relPath)
	return Image{
		URL:           imageURL,
		URLBase:       baseURL + strings.TrimSuffix(URLPath(relPath), path.Ext(relPath)),
		Copyright:     copyright(title, r.Copyright),
		CopyrightLink: imageURL,
		Title:         title,
		StartDate:     date.Format(dateLayout),
		FullStartDate: date.Format(dateLayout) + "0000",
		EndDate:       date.Format(dateLayout),
		WP:            true,
		HSH:           hash(relPath),
		Drk:           0,
		Top:           0,
		Bot:           0,
		Quiz:          "",
	}
}

func copyright(title, label string) string {
	if label == "" {
		return title
	}
	return title + " (\u00a9 " + label + ")"
}

// hash returns a deterministic short digest of the image path.
func hash(relPath string) string {
	sum := sha1.Sum([]byte(relPath))
	return hex.EncodeToString(sum[:])[:16]
}
