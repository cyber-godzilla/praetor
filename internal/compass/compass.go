package compass

import (
	"image"
	"image/color"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Rows is the number of terminal rows for compass display.
const Rows = 7

// View returns placeholder text for layout.
func View(width int) string {
	var buf strings.Builder
	for i := 0; i < Rows; i++ {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(" ", width))
	}
	return buf.String()
}

// BuildImage renders the compass to an RGBA image.
func BuildImage(exits types.Exits, width int) *image.RGBA {
	imgW := width * 5
	imgH := Rows * 10

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	bg := color.RGBA{18, 18, 24, 255}
	for y := 0; y < imgH; y++ {
		for x := 0; x < imgW; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	active := color.RGBA{85, 204, 85, 255}
	inactive := color.RGBA{40, 40, 50, 255}
	centerCol := color.RGBA{232, 168, 56, 255}

	cellW := imgW / 3
	cellH := imgH / 3

	pick := func(on bool) color.RGBA {
		if on {
			return active
		}
		return inactive
	}

	// Arrow size relative to cell.
	sz := cellH / 2
	if sz < 6 {
		sz = 6
	}

	// ---- Cardinal arrows (filled triangles) ----

	// North: triangle pointing up
	drawArrowUp(img, cellW+cellW/2, cellH/2, sz, pick(exits.North))
	// South: triangle pointing down
	drawArrowDown(img, cellW+cellW/2, cellH*2+cellH/2, sz, pick(exits.South))
	// West: triangle pointing left
	drawArrowLeft(img, cellW/2, cellH+cellH/2, sz, pick(exits.West))
	// East: triangle pointing right
	drawArrowRight(img, cellW*2+cellW/2, cellH+cellH/2, sz, pick(exits.East))

	// ---- Diagonal arrows (slightly smaller than cardinals) ----
	dsz := sz * 4 / 5
	if dsz < 5 {
		dsz = 5
	}

	// NW: arrow pointing upper-left
	drawArrowUpLeft(img, cellW/2, cellH/2, dsz, pick(exits.Northwest))
	// NE: arrow pointing upper-right
	drawArrowUpRight(img, cellW*2+cellW/2, cellH/2, dsz, pick(exits.Northeast))
	// SW: arrow pointing lower-left
	drawArrowDownLeft(img, cellW/2, cellH*2+cellH/2, dsz, pick(exits.Southwest))
	// SE: arrow pointing lower-right
	drawArrowDownRight(img, cellW*2+cellW/2, cellH*2+cellH/2, dsz, pick(exits.Southeast))

	// ---- Center: up/down indicators ----
	cx := cellW + cellW/2
	cy := cellH + cellH/2

	// Center dot
	dotR := sz / 5
	if dotR < 2 {
		dotR = 2
	}
	fillCircle(img, cx, cy, dotR, centerCol)

	// Up/down indicators (same size as diagonals, orange like center dot)
	if exits.Up {
		drawArrowUp(img, cx, cy-cellH/3, dsz, centerCol)
	}
	if exits.Down {
		drawArrowDown(img, cx, cy+cellH/3, dsz, centerCol)
	}

	return img
}

// Render returns a layout placeholder and an encoded graphics escape for
// the compass in the given mode. ModeNone returns a text fallback and no
// escape.
func Render(mode graphics.Mode, exits types.Exits, width int) (placeholder string, escape string) {
	if mode == graphics.ModeNone {
		return compassFallback(width), ""
	}
	img := BuildImage(exits, width)
	return View(width), graphics.Encode(mode, img, width, Rows)
}

func compassFallback(width int) string {
	msg := "Compass unavailable"
	if len(msg) > width {
		msg = msg[:width]
	}
	pad := width - len(msg)
	line := msg + strings.Repeat(" ", pad)
	lines := make([]string, Rows)
	mid := Rows / 2
	for i := range lines {
		if i == mid {
			lines[i] = line
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
	}
	return strings.Join(lines, "\n")
}


// ---- Arrow drawing functions ----

func drawArrowUp(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := 1 + row*2
		left := cx - w/2
		y := cy - size/2 + row
		for x := left; x < left+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowDown(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := 1 + (size-1-row)*2
		left := cx - w/2
		y := cy - size/2 + row
		for x := left; x < left+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowLeft(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for col := 0; col < size; col++ {
		h := 1 + col*2
		top := cy - h/2
		x := cx - size/2 + col
		for y := top; y < top+h; y++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowRight(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for col := 0; col < size; col++ {
		h := 1 + (size-1-col)*2
		top := cy - h/2
		x := cx - size/2 + col
		for y := top; y < top+h; y++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowUpLeft(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := size - row
		x0 := cx - size/2
		y := cy - size/2 + row
		for x := x0; x < x0+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowUpRight(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := size - row
		x0 := cx + size/2 - w
		y := cy - size/2 + row
		for x := x0; x < x0+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowDownLeft(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := row + 1
		x0 := cx - size/2
		y := cy - size/2 + row
		for x := x0; x < x0+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func drawArrowDownRight(img *image.RGBA, cx, cy, size int, c color.RGBA) {
	bounds := img.Bounds()
	for row := 0; row < size; row++ {
		w := row + 1
		x0 := cx + size/2 - w
		y := cy - size/2 + row
		for x := x0; x < x0+w; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	bounds := img.Bounds()
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				x, y := cx+dx, cy+dy
				if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}
