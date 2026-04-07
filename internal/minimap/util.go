package minimap

import (
	"image/color"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// FindPlayerRoom returns the index of the player room (red-colored).
// Returns -1 if no rooms provided, 0 if no red room found.
func FindPlayerRoom(rooms []types.MinimapRoom) int {
	if len(rooms) == 0 {
		return -1
	}
	for i, r := range rooms {
		cr, cg, cb := hexToRGB(r.Color)
		if cr > 200 && cg < 50 && cb < 50 {
			return i
		}
	}
	return 0
}

// orchilColor applies Orchil's brightness formula to a hex color.
func orchilColor(hexColor string, brightness float64) color.RGBA {
	r, g, b := hexToRGB(hexColor)
	lig := brightness
	if lig > 30 {
		lig = 30
	}
	if lig < 0 {
		lig = 0
	}
	adj := int((lig - 25) * 8)
	return color.RGBA{
		R: clamp8(int(r) + adj),
		G: clamp8(int(g) + adj),
		B: clamp8(int(b) + adj),
		A: 255,
	}
}

func hexToRGB(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 128, 128, 128
	}
	r := hexVal(hex[0])*16 + hexVal(hex[1])
	g := hexVal(hex[2])*16 + hexVal(hex[3])
	b := hexVal(hex[4])*16 + hexVal(hex[5])
	return uint8(r), uint8(g), uint8(b)
}

func hexVal(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return 0
	}
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
