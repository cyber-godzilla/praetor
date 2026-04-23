package minimap

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func sampleRooms() []types.MinimapRoom {
	return []types.MinimapRoom{
		{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25},
		{X: 20, Y: 0, Size: 10, Color: "#ffffff", Brightness: 22},
	}
}

func TestBuildImage_NonNilWithRooms(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	m.Update(sampleRooms(), nil)
	if img := m.BuildImage(); img == nil {
		t.Fatal("BuildImage returned nil with rooms present")
	}
}

func TestBuildImage_NilWithNoRooms(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	if img := m.BuildImage(); img != nil {
		t.Fatal("BuildImage should return nil when no rooms are loaded")
	}
}

func TestRender_ModeKitty_ProducesEscape(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	m.Update(sampleRooms(), nil)
	placeholder, esc := m.Render(graphics.ModeKitty)
	if placeholder == "" {
		t.Error("expected non-empty placeholder for layout")
	}
	if !strings.HasPrefix(esc, "\x1b_G") {
		t.Errorf("expected kitty APC escape, got %q", esc[:mnInt(4, len(esc))])
	}
}

func TestRender_ModeNone_ReturnsFallback(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	m.Update(sampleRooms(), nil)
	placeholder, esc := m.Render(graphics.ModeNone)
	if esc != "" {
		t.Errorf("expected empty escape for ModeNone, got %d bytes", len(esc))
	}
	if !strings.Contains(placeholder, "Minimap unavailable") {
		t.Errorf("expected fallback text in placeholder, got:\n%s", placeholder)
	}
}

func mnInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
