package compass

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func sampleExits() types.Exits {
	return types.Exits{North: true, East: true, South: false, West: true}
}

func TestBuildImage_NonNil(t *testing.T) {
	if img := BuildImage(sampleExits(), 20); img == nil {
		t.Fatal("BuildImage returned nil")
	}
}

func TestRender_ModeKitty_ProducesEscape(t *testing.T) {
	placeholder, esc := Render(graphics.ModeKitty, sampleExits(), 20, 2)
	if placeholder == "" {
		t.Error("expected non-empty placeholder")
	}
	if !strings.HasPrefix(esc, "\x1b_G") {
		t.Errorf("expected kitty APC escape, got prefix %q", esc[:compassMinInt(4, len(esc))])
	}
	if !strings.Contains(esc, "i=2") {
		t.Errorf("expected i=2 in escape for image-id replace-in-place, got: %q", esc[:compassMinInt(80, len(esc))])
	}
}

func TestRender_ModeNone_ReturnsFallbackText(t *testing.T) {
	placeholder, esc := Render(graphics.ModeNone, sampleExits(), 20, 2)
	if esc != "" {
		t.Errorf("expected empty escape for ModeNone, got %d bytes", len(esc))
	}
	if !strings.Contains(placeholder, "Compass unavailable") {
		t.Errorf("expected 'Compass unavailable' in placeholder, got:\n%s", placeholder)
	}
}

func compassMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
