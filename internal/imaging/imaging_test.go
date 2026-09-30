package imaging

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestDimensionsReadsPNGHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	w, h, err := Dimensions(&buf, "png")
	if err != nil {
		t.Fatalf("Dimensions: %v", err)
	}
	if w != 3 || h != 2 {
		t.Fatalf("Dimensions = %dx%d, want 3x2", w, h)
	}
}

func TestDimensionsRejectsGarbage(t *testing.T) {
	if _, _, err := Dimensions(bytes.NewReader([]byte("not an image")), "png"); err == nil {
		t.Fatal("Dimensions accepted a non-image stream")
	}
}
