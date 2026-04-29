package graphics

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func makeTestImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 255, 0, 255})
		}
	}
	return img
}

func TestEncode_ModeNone_ReturnsEmpty(t *testing.T) {
	out := Encode(ModeNone, makeTestImage(), 2, 2, 0)
	if out != "" {
		t.Fatalf("expected empty string for ModeNone, got %d bytes", len(out))
	}
}

func TestEncode_ModeKitty_ReturnsKittyEscape(t *testing.T) {
	out := Encode(ModeKitty, makeTestImage(), 2, 2, 1)
	if !strings.HasPrefix(out, "\x1b_G") {
		t.Fatalf("expected kitty APC prefix \\x1b_G, got %q", out[:min(4, len(out))])
	}
	if !strings.Contains(out, "i=1") {
		t.Errorf("expected i=1 in kitty escape (image id), got: %q", out[:min(80, len(out))])
	}
}

func TestEncode_ModeKitty_ZeroIDOmitsAttribute(t *testing.T) {
	out := Encode(ModeKitty, makeTestImage(), 2, 2, 0)
	if strings.Contains(out, "i=") {
		t.Errorf("expected no i= attribute when imageID=0, got: %q", out[:min(80, len(out))])
	}
}

func TestEncode_ModeSixel_ReturnsSixelEscape(t *testing.T) {
	out := Encode(ModeSixel, makeTestImage(), 2, 2, 0)
	if !strings.HasPrefix(out, "\x1bP") {
		t.Fatalf("expected sixel DCS prefix \\x1bP, got %q", out[:min(4, len(out))])
	}
}

func TestModeString(t *testing.T) {
	cases := map[Mode]string{
		ModeNone:  "none",
		ModeSixel: "sixel",
		ModeKitty: "kitty",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("Mode(%d).String() = %q, want %q", m, got, want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
