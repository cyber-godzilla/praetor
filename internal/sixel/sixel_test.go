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

func TestEncode_LargerCellAllocationProducesLongerOutput(t *testing.T) {
	// Larger cell allocation → larger target pixel dimensions → more
	// sixel payload. Confirms the resize-to-fit math actually scales
	// based on the cols/rows arguments.
	small := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			small.SetRGBA(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	smallOut := Encode(small, 2, 2)
	largeOut := Encode(small, 20, 12)
	if len(smallOut) == 0 || len(largeOut) == 0 {
		t.Fatal("Encode returned empty string")
	}
	if len(largeOut) <= len(smallOut) {
		t.Errorf("expected larger cell allocation to produce longer payload; got small=%d large=%d", len(smallOut), len(largeOut))
	}
}

func TestEncode_ZeroDimensionPreservesInput(t *testing.T) {
	// resize is internal; degenerate cell counts (0 cols or 0 rows)
	// short-circuit to the original image. Verify Encode still
	// produces a payload (rather than nil-deref or empty string).
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	out := Encode(img, 0, 0)
	if !strings.HasPrefix(out, "\x1bP") {
		t.Errorf("expected DCS prefix even with zero allocation, got %q", out[:min(4, len(out))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
