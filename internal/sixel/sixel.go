// Package sixel encodes RGBA images as Sixel escape sequences.
// Wraps github.com/mattn/go-sixel so callers see a stable surface.
package sixel

import (
	"bytes"
	"image"

	gosixel "github.com/mattn/go-sixel"
	xdraw "golang.org/x/image/draw"
)

// Sixel renders pixels at native image size — there is no protocol-
// level "scale to N cells" hint like kitty's c=/r=. To keep the image
// inside the cell area lipgloss reserved for it, we resize the source
// image to (cols × cellPxW) × (rows × cellPxH) before encoding.
//
// The constants below are intentionally smaller than the cell pixel
// dimensions of any common Linux terminal font (which typically run
// 8-10 wide × 14-22 tall). Picking conservative values means the
// image lands inside its cell allocation with some margin even on
// terminals with the smallest cells; the cost is a visible gap on
// terminals with larger cells. Overflow would push surrounding
// sidebar content out of view, which is the worse failure mode.
const (
	cellPxW = 6
	cellPxH = 12
)

// Encode renders an RGBA image as a Sixel DCS escape sequence.
// cols and rows specify the cell footprint the caller wants the image
// to occupy; the source image is resized so the rendered pixels stay
// within that allocation.
func Encode(img *image.RGBA, cols, rows int) string {
	src := resize(img, cols*cellPxW, rows*cellPxH)
	var buf bytes.Buffer
	enc := gosixel.NewEncoder(&buf)
	if err := enc.Encode(src); err != nil {
		return ""
	}
	return buf.String()
}

// resize returns a copy of src scaled to w × h via nearest-neighbor
// sampling. Preserves the pixel-art look of the minimap and keeps
// single-pixel walls crisp. If the target dimensions are non-positive
// the original is returned unchanged.
func resize(src *image.RGBA, w, h int) *image.RGBA {
	if w <= 0 || h <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}
