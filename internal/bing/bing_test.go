package bing

import (
	"strings"
	"testing"
	"time"
)

func TestRenderFields(t *testing.T) {
	r := Renderer{Copyright: "My Wallpapers"}
	date := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	img := r.Render("https://wall.example.com", "landscapes/lake.jpg", "lake", date)

	if img.URL != "https://wall.example.com/images/landscapes/lake.jpg" {
		t.Fatalf("URL = %q", img.URL)
	}
	if img.URLBase != "https://wall.example.com/images/landscapes/lake" {
		t.Fatalf("URLBase = %q", img.URLBase)
	}
	if img.Copyright != "lake (\u00a9 My Wallpapers)" {
		t.Fatalf("Copyright = %q", img.Copyright)
	}
	if img.CopyrightLink != img.URL {
		t.Fatalf("CopyrightLink = %q, want %q", img.CopyrightLink, img.URL)
	}
	if img.Title != "lake" {
		t.Fatalf("Title = %q", img.Title)
	}
	if img.StartDate != "20260930" || img.EndDate != "20260930" {
		t.Fatalf("dates = %q / %q", img.StartDate, img.EndDate)
	}
	if img.FullStartDate != "202609300000" {
		t.Fatalf("FullStartDate = %q", img.FullStartDate)
	}
	if !img.WP {
		t.Fatal("WP must be true")
	}
	if len(img.HSH) != 16 {
		t.Fatalf("HSH = %q, want 16 chars", img.HSH)
	}
	if img.Drk != 0 || img.Top != 0 || img.Bot != 0 || img.Quiz != "" {
		t.Fatalf("unexpected defaults: %+v", img)
	}
}

func TestRenderHashIsDeterministic(t *testing.T) {
	r := Renderer{Copyright: "x"}
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := r.Render("https://a", "p/q.jpg", "q", date)
	b := r.Render("https://a", "p/q.jpg", "q", date)
	if a.HSH != b.HSH {
		t.Fatalf("hash not deterministic: %q vs %q", a.HSH, b.HSH)
	}
	c := r.Render("https://a", "p/r.jpg", "r", date)
	if a.HSH == c.HSH {
		t.Fatal("different paths produced the same hash")
	}
}

func TestRenderEmptyCopyrightLabel(t *testing.T) {
	r := Renderer{}
	img := r.Render("https://a", "x.jpg", "x", time.Now())
	if img.Copyright != "x" {
		t.Fatalf("Copyright = %q, want %q", img.Copyright, "x")
	}
}

func TestURLPathCleansInput(t *testing.T) {
	cases := map[string]string{
		"a/b.jpg":       "/images/a/b.jpg",
		"nested/c.webp": "/images/nested/c.webp",
		"c.jpg":         "/images/c.jpg",
	}
	for in, want := range cases {
		if got := URLPath(in); got != want {
			t.Fatalf("URLPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestURLPathDoesNotEscape(t *testing.T) {
	if got := URLPath("../../etc/passwd"); strings.Contains(got, "..") {
		t.Fatalf("URLPath escaped the root: %q", got)
	}
}
