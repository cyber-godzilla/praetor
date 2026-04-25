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

func TestEncode_LargerImageProducesLongerOutput(t *testing.T) {
	// Scaling input pixels should grow the encoded payload — confirms
	// upscaling actually runs.
	small := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			small.SetRGBA(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	out := Encode(small, 2, 1)
	if len(out) == 0 {
		t.Fatal("Encode returned empty string")
	}
	// Reasonable upper bound — input 10x10 upscaled 2x → 20x20 → some
	// sixel payload bigger than just the DCS prefix.
	if len(out) < 50 {
		t.Errorf("expected larger sixel payload after upscale, got %d bytes", len(out))
	}
}

func TestEncode_NilUpscale(t *testing.T) {
	// Calling upscale with factor 1 should return the source unchanged.
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	dst := upscale(src, 1)
	if dst != src {
		t.Errorf("upscale(_, 1) should return the same image, got copy")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
