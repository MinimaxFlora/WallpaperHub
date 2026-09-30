package manifest

import "testing"

const validDocument = `{
  "version": 1,
  "generated_at": "2026-09-30T00:00:00Z",
  "timezone": "Asia/Shanghai",
  "count": 0,
  "images": [
    {"id":"02","path":"images/02.webp","title":"Two","category":"landscape","tags":null,"width":1080,"height":1920,"format":"webp","bytes":8,"hash":"sha256-bbbb"},
    {"id":"01","path":"images/01.webp","title":"One","category":"anime","tags":["anime"],"width":1920,"height":1080,"orientation":"landscape","format":"webp","bytes":8,"hash":"sha256-aaaa"}
  ]
}`

func TestParseSortsAndFillsDefaults(t *testing.T) {
	m, err := Parse([]byte(validDocument))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Count != 2 {
		t.Fatalf("count = %d, want 2", m.Count)
	}
	if m.Images[0].ID != "01" || m.Images[1].ID != "02" {
		t.Fatalf("images are not sorted by id: %+v", m.Images)
	}
	if got := m.Images[1].Orientation; got != "portrait" {
		t.Fatalf("derived orientation = %q, want portrait", got)
	}
	if m.Images[1].Tags == nil {
		t.Fatal("nil tags were not normalized to an empty slice")
	}
}

func TestParseRejectsInvalidDocuments(t *testing.T) {
	cases := map[string]string{
		"missing version": `{"images":[{"id":"01","path":"p","hash":"h"}]}`,
		"duplicate id":    `{"version":1,"images":[{"id":"01","path":"a","hash":"h"},{"id":"01","path":"b","hash":"h"}]}`,
		"empty path":      `{"version":1,"images":[{"id":"01","path":"","hash":"h"}]}`,
		"empty hash":      `{"version":1,"images":[{"id":"01","path":"a","hash":""}]}`,
		"not json":        `not-json`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatal("Parse accepted an invalid document")
			}
		})
	}
}

func TestFind(t *testing.T) {
	m, err := Parse([]byte(validDocument))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if img, ok := m.Find("02"); !ok || img.Title != "Two" {
		t.Fatalf("Find(02) = %+v, %v", img, ok)
	}
	if _, ok := m.Find("99"); ok {
		t.Fatal("Find(99) reported a match")
	}
}

func TestFormatAndOrientation(t *testing.T) {
	if got := Format("images/01.WEBP"); got != "webp" {
		t.Errorf("Format() = %q, want webp", got)
	}
	if got := Format("images/01.txt"); got != "" {
		t.Errorf("Format() = %q, want empty", got)
	}
	if IsImage("notes.md") {
		t.Error("IsImage accepted a non-image")
	}
	if got := Orientation(100, 200); got != "portrait" {
		t.Errorf("Orientation() = %q, want portrait", got)
	}
	if got := Orientation(200, 200); got != "square" {
		t.Errorf("Orientation() = %q, want square", got)
	}
}
