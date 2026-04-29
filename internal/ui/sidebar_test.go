package ui

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestSidebar_ModeNone_RendersFallback(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)

	mm, cp := s.ConsumeGraphics()
	if mm != "" || cp != "" {
		t.Errorf("expected empty escapes in ModeNone, got mm=%d bytes cp=%d bytes", len(mm), len(cp))
	}

	view := s.View()
	if !strings.Contains(view, "Minimap unavailable") {
		t.Errorf("expected sidebar view to contain fallback text, got:\n%s", view)
	}
	if !strings.Contains(view, "Compass unavailable") {
		t.Errorf("expected sidebar view to contain 'Compass unavailable', got:\n%s", view)
	}
}

func TestSidebar_Kitty_ConsumeReturnsOnEveryCall(t *testing.T) {
	// kitty path: re-emitting at the same image id is an atomic
	// in-place replace (no flicker), so ConsumeGraphics returns the
	// escapes on every call — even when nothing changed. This is what
	// makes the path self-healing if the terminal silently loses the
	// image.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)

	mm1, _ := s.ConsumeGraphics()
	if mm1 == "" {
		t.Fatal("expected first ConsumeGraphics to return minimap escape")
	}
	mm2, _ := s.ConsumeGraphics()
	if mm2 != mm1 {
		t.Errorf("expected second ConsumeGraphics to return same escape; got different bytes")
	}
}

func TestSidebar_Sixel_ConsumeReturnsOnEveryCall(t *testing.T) {
	// sixel path: pixels live inline in cells, so any surrounding
	// text write overwrites them. ConsumeGraphics returns the cached
	// escape on every call (not just dirty ones) so the image stays
	// visible across frame redraws.
	s := NewSidebar(0.8, 12, graphics.ModeSixel)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)

	mm1, _ := s.ConsumeGraphics()
	if mm1 == "" {
		t.Fatal("expected first ConsumeGraphics to return sixel escape")
	}
	mm2, _ := s.ConsumeGraphics()
	if mm2 != mm1 {
		t.Errorf("expected sixel escape to be returned on every call; got different bytes")
	}
}

func TestSidebar_HideGraphics_KittyEmitsDeletes(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	_, _ = s.ConsumeGraphics() // mark as emitted

	hide := s.HideGraphics()
	if !strings.Contains(hide, "a=d,d=I,i=1") {
		t.Errorf("expected delete-by-id for minimap (i=1), got: %q", hide)
	}
	if !strings.Contains(hide, "a=d,d=I,i=2") {
		t.Errorf("expected delete-by-id for compass (i=2), got: %q", hide)
	}

	// Idempotent: second call returns "".
	if got := s.HideGraphics(); got != "" {
		t.Errorf("expected second HideGraphics to be no-op, got %d bytes", len(got))
	}
}

func TestSidebar_HideGraphics_NoOpWhenNothingEmitted(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	if got := s.HideGraphics(); got != "" {
		t.Errorf("expected HideGraphics on fresh sidebar to be no-op, got %d bytes", len(got))
	}
}

func TestSidebar_InvalidateGraphics_ResetsEmittedFlag(t *testing.T) {
	// After Invalidate, HideGraphics should be a no-op (terminal
	// images are already gone) and ConsumeGraphics keeps emitting.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	_, _ = s.ConsumeGraphics() // mark emitted
	s.InvalidateGraphics()
	if got := s.HideGraphics(); got != "" {
		t.Errorf("expected HideGraphics after Invalidate to be no-op, got %d bytes", len(got))
	}
	if mm, _ := s.ConsumeGraphics(); mm == "" {
		t.Errorf("expected ConsumeGraphics to keep returning escapes")
	}
}
