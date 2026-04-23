// Package sixel encodes RGBA images as Sixel escape sequences.
// Wraps github.com/mattn/go-sixel so callers see a stable surface.
package sixel

import (
	"bytes"
	"image"

	gosixel "github.com/mattn/go-sixel"
)

// Encode renders an RGBA image as a Sixel DCS escape sequence.
// cols and rows are accepted for parity with other encoders but Sixel
// self-sizes from the image dimensions — they are ignored.
func Encode(img *image.RGBA, cols, rows int) string {
	_ = cols
	_ = rows
	var buf bytes.Buffer
	enc := gosixel.NewEncoder(&buf)
	if err := enc.Encode(img); err != nil {
		return ""
	}
	return buf.String()
}
