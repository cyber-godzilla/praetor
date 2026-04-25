// Package sixel encodes RGBA images as Sixel escape sequences.
// Wraps github.com/mattn/go-sixel so callers see a stable surface.
package sixel

import (
	"bytes"
	"image"

	gosixel "github.com/mattn/go-sixel"
	xdraw "golang.org/x/image/draw"
)

// upscaleFactor controls how much the source image is enlarged before
// sixel encoding. The minimap source is cols*5 × rows*10 px; doubling
// brings rendered output to ~full cell allocation on terminals with
// typical 10x20-pixel cells (Windows Terminal, foot, mintty, xterm).
const upscaleFactor = 2

// Encode renders an RGBA image as a Sixel DCS escape sequence.
// cols and rows are accepted for parity with other encoders. Sixel
// renders at the source image's raw pixel size, so we upscale to better
// fill the cell area lipgloss reserves on the terminal grid.
func Encode(img *image.RGBA, cols, rows int) string {
	_ = cols
	_ = rows
	var buf bytes.Buffer
	enc := gosixel.NewEncoder(&buf)
	src := upscale(img, upscaleFactor)
	if err := enc.Encode(src); err != nil {
		return ""
	}
	return buf.String()
}

// upscale returns a copy of src enlarged by an integer factor using
// nearest-neighbor sampling. Preserves the pixel-art look of the
// minimap and keeps single-pixel walls crisp.
func upscale(src *image.RGBA, factor int) *image.RGBA {
	if factor <= 1 {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*factor, b.Dy()*factor))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	return dst
}
