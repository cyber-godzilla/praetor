package minimap

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/kitty"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Minimap renders the minimap using direct SKOOT coordinates with room
// outlines and direct wall rendering (no adjacency detection).
type Minimap struct {
	rooms  []types.MinimapRoom
	walls  []types.MinimapWall
	width  int // display columns
	height int // display rows
	scale  float64
}

func NewMinimap() Minimap {
	return Minimap{width: 38, height: 12, scale: 0.8}
}

func (m *Minimap) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Minimap) SetScale(s float64) {
	m.scale = s
}

func (m *Minimap) Update(rooms []types.MinimapRoom, walls []types.MinimapWall) {
	if rooms != nil {
		m.rooms = rooms
	}
	if walls != nil {
		m.walls = walls
	}
}

func (m Minimap) Render() (placeholder string, kittyEscape string) {
	playerIdx := FindPlayerRoom(m.rooms)
	if playerIdx < 0 || m.width < 4 || m.height < 2 {
		return "", ""
	}

	imgW := m.width * 5
	imgH := m.height * 10

	scale := m.computeScale(imgW, imgH, playerIdx)

	// Center on player room. SKOOT x,y are direct positions;
	// we offset so the player room's top-left maps to canvas center.
	playerRoom := m.rooms[playerIdx]
	playerCX := float64(playerRoom.X) + float64(playerRoom.Size)/2.0
	playerCY := float64(playerRoom.Y) + float64(playerRoom.Size)/2.0
	centerPX := float64(imgW) / 2.0
	centerPY := float64(imgH) / 2.0

	toPX := func(gx float64) float64 { return (gx-playerCX)*scale + centerPX }
	toPY := func(gy float64) float64 { return (gy-playerCY)*scale + centerPY }

	// Create image with black background.
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	bg := color.RGBA{10, 10, 14, 255}
	for y := 0; y < imgH; y++ {
		for x := 0; x < imgW; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	// Draw rooms: fill at full SKOOT width, then 1px black outline.
	for _, room := range m.rooms {
		px := int(toPX(float64(room.X)))
		py := int(toPY(float64(room.Y)))
		pw := int(math.Round(float64(room.Size) * scale))
		if pw < 1 {
			pw = 1
		}

		fill := orchilColor(room.Color, room.Brightness)

		// Filled rectangle.
		for y := py; y < py+pw; y++ {
			for x := px; x < px+pw; x++ {
				setRGBA(img, x, y, fill)
			}
		}

		// 1px black outline on top.
		outline := color.RGBA{0, 0, 0, 255}
		for x := px; x < px+pw; x++ {
			setRGBA(img, x, py, outline)      // top
			setRGBA(img, x, py+pw-1, outline) // bottom
		}
		for y := py; y < py+pw; y++ {
			setRGBA(img, px, y, outline)      // left
			setRGBA(img, px+pw-1, y, outline) // right
		}
	}

	// Draw walls/passages using SKOOT 10 coordinates directly.
	// Only draw walls when there are 2+ rooms — with a single room,
	// wall lines are just noise radiating into empty space.
	if len(m.rooms) < 2 {
		return m.finishRender(img)
	}

	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}

	lineOffset := int(math.Max(2, math.Round(scale*2.5)))
	diagOffset := int(math.Max(3, math.Round(scale*3.5)))
	lineThick := int(math.Max(1, math.Round(scale*1.5)))

	for _, wall := range m.walls {
		wx := int(toPX(float64(wall.X)))
		wy := int(toPY(float64(wall.Y)))

		dir := wall.Type
		accessible := wall.Passable

		// Normalize "no" prefix: none→ne blocked, nonw→nw blocked.
		if len(dir) > 2 && dir[:2] == "no" {
			dir = dir[2:]
			accessible = false
		}

		c := black
		thick := lineThick + 1
		if accessible {
			c = white
			thick = lineThick
		}

		switch dir {
		case "hor":
			drawLineV2(img, wx-lineOffset, wy, wx+lineOffset, wy, thick, c)
		case "ver":
			drawLineV2(img, wx, wy-lineOffset, wx, wy+lineOffset, thick, c)
		case "ne":
			drawLineV2(img, wx+diagOffset, wy-diagOffset, wx-diagOffset, wy+diagOffset, thick, c)
		case "nw":
			drawLineV2(img, wx-diagOffset, wy-diagOffset, wx+diagOffset, wy+diagOffset, thick, c)
		}
	}

	return m.finishRender(img)
}

func (m Minimap) finishRender(img *image.RGBA) (string, string) {
	kittyEscape := kitty.Encode(img, m.width, m.height)

	var buf strings.Builder
	for i := 0; i < m.height; i++ {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(" ", m.width))
	}

	return buf.String(), kittyEscape
}

func (m Minimap) computeScale(imgW, imgH, playerIdx int) float64 {
	scale := m.scale

	maxRoomSize := 0
	for _, r := range m.rooms {
		if r.Size > maxRoomSize {
			maxRoomSize = r.Size
		}
	}

	// Boost for small rooms.
	if maxRoomSize <= 10 {
		scale *= 2
	}

	// Cap 1: largest room at 40% of canvas.
	if maxRoomSize > 0 {
		roomCap := float64(imgW) * 0.4 / float64(maxRoomSize)
		if roomCap < scale {
			scale = roomCap
		}
	}

	// Cap 2: nearby rooms fit in canvas.
	playerRoom := m.rooms[playerIdx]
	nearRadius := maxRoomSize * 3
	if nearRadius < 60 {
		nearRadius = 60
	}
	minX, minY := playerRoom.X, playerRoom.Y
	maxX, maxY := playerRoom.X+playerRoom.Size, playerRoom.Y+playerRoom.Size
	for _, r := range m.rooms {
		dx := r.X - playerRoom.X
		dy := r.Y - playerRoom.Y
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		if dx > nearRadius || dy > nearRadius {
			continue
		}
		if r.X < minX {
			minX = r.X
		}
		if r.X+r.Size > maxX {
			maxX = r.X + r.Size
		}
		if r.Y < minY {
			minY = r.Y
		}
		if r.Y+r.Size > maxY {
			maxY = r.Y + r.Size
		}
	}
	spanX := maxX - minX
	spanY := maxY - minY
	margin := 10
	if spanX > 0 {
		fitX := float64(imgW-margin*2) / float64(spanX)
		if fitX < scale {
			scale = fitX
		}
	}
	if spanY > 0 {
		fitY := float64(imgH-margin*2) / float64(spanY)
		if fitY < scale {
			scale = fitY
		}
	}

	return scale
}

// drawLineV2 draws a line between two points with given thickness.
func drawLineV2(img *image.RGBA, x1, y1, x2, y2, thick int, c color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	steps := abs(dx)
	if abs(dy) > steps {
		steps = abs(dy)
	}
	if steps == 0 {
		for t := -thick / 2; t <= thick/2; t++ {
			setRGBA(img, x1+t, y1, c)
			setRGBA(img, x1, y1+t, c)
		}
		return
	}

	halfT := thick / 2
	for i := 0; i <= steps; i++ {
		x := x1 + i*dx/steps
		y := y1 + i*dy/steps
		for tx := -halfT; tx <= halfT; tx++ {
			for ty := -halfT; ty <= halfT; ty++ {
				setRGBA(img, x+tx, y+ty, c)
			}
		}
	}
}

func setRGBA(img *image.RGBA, x, y int, c color.RGBA) {
	bounds := img.Bounds()
	if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
		img.SetRGBA(x, y, c)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
