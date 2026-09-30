// Package imaging extracts the pixel dimensions of an image stream.
package imaging

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"

	// Register the decoders used by image.DecodeConfig.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// maxProbeBytes bounds how much of an image is buffered for dimension probing.
const maxProbeBytes = 8 << 20

// Dimensions returns the pixel width and height of an image stream. AVIF is
// handled by a minimal box parser because the standard library cannot decode
// it; other formats go through image.DecodeConfig.
func Dimensions(r io.Reader, format string) (int, int, error) {
	if format == "avif" {
		return avifDimensions(r)
	}
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// avifDimensions scans the ISOBMFF boxes for an ispe (image spatial extents)
// box and returns the largest one, which is normally the primary image.
func avifDimensions(r io.Reader) (int, int, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxProbeBytes))
	if err != nil {
		return 0, 0, err
	}
	width, height := 0, 0
	for offset := 0; offset+12 <= len(data); {
		idx := bytes.Index(data[offset:], []byte("ispe"))
		if idx < 0 {
			break
		}
		// Box layout: [size 4][type "ispe"][version 1][flags 3][width 4][height 4].
		values := offset + idx + 8
		if values+8 > len(data) {
			break
		}
		w := int(binary.BigEndian.Uint32(data[values:]))
		h := int(binary.BigEndian.Uint32(data[values+4:]))
		if w > 0 && h > 0 && w*h > width*height {
			width, height = w, h
		}
		offset += idx + 4
	}
	if width == 0 || height == 0 {
		return 0, 0, fmt.Errorf("avif: no ispe box found")
	}
	return width, height, nil
}
