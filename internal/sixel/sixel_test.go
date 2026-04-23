package sixel

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestEncode_NonEmpty(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	out := Encode(img, 2, 1)
	if out == "" {
		t.Fatal("Encode returned empty string")
	}
	if !strings.HasPrefix(out, "\x1bP") {
		t.Fatalf("expected DCS prefix \\x1bP, got %q", out[:min(4, len(out))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
