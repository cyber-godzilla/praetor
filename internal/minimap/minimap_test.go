package minimap

import (
	"math"
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

func TestComputeScale_UserScaleMultipliesAutoFitZoom(t *testing.T) {
	m := NewMinimap()
	m.SetSize(38, 12)
	m.Update([]types.MinimapRoom{
		{X: 0, Y: 0, Size: 100, Color: "#ff0000", Brightness: 25},
		{X: 50, Y: 0, Size: 100, Color: "#ffffff", Brightness: 22},
	}, nil)
	player := FindPlayerRoom(m.rooms)

	m.SetScale(0.8)
	zoomedOut := m.computeScale(190, 120, player)
	m.SetScale(2.0)
	zoomedIn := m.computeScale(190, 120, player)

	if zoomedIn <= zoomedOut {
		t.Fatalf("scale 2.0 produced %f, want greater than scale 0.8 (%f)", zoomedIn, zoomedOut)
	}
	wantRatio := 2.0 / 0.8
	if got := zoomedIn / zoomedOut; math.Abs(got-wantRatio) > 0.001 {
		t.Fatalf("effective zoom ratio = %f, want %f", got, wantRatio)
	}
}

func TestRender_ModeKitty_ProducesEscape(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	m.Update(sampleRooms(), nil)
	placeholder, esc := m.Render(graphics.ModeKitty, 1)
	if placeholder == "" {
		t.Error("expected non-empty placeholder for layout")
	}
	if !strings.HasPrefix(esc, "\x1b_G") {
		t.Errorf("expected kitty APC escape, got %q", esc[:mnInt(4, len(esc))])
	}
	if !strings.Contains(esc, "i=1") {
		t.Errorf("expected i=1 in escape for image-id replace-in-place, got: %q", esc[:mnInt(80, len(esc))])
	}
}

func TestRender_ModeNone_ReturnsFallback(t *testing.T) {
	m := NewMinimap()
	m.SetSize(40, 12)
	m.Update(sampleRooms(), nil)
	placeholder, esc := m.Render(graphics.ModeNone, 1)
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
