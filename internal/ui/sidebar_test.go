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

	_, mm, cp := s.ConsumeGraphics("sidebar")
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

	_, mm1, _ := s.ConsumeGraphics("sidebar")
	if mm1 == "" {
		t.Fatal("expected first ConsumeGraphics to return minimap escape")
	}
	_, mm2, _ := s.ConsumeGraphics("sidebar")
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

	_, mm1, _ := s.ConsumeGraphics("sidebar")
	if mm1 == "" {
		t.Fatal("expected first ConsumeGraphics to return sixel escape")
	}
	_, mm2, _ := s.ConsumeGraphics("sidebar")
	if mm2 != mm1 {
		t.Errorf("expected sixel escape to be returned on every call; got different bytes")
	}
}

func TestSidebar_AnchorChangeEmitsTransitionDelete(t *testing.T) {
	// kitty creates a NEW placement on each `a=T` rather than moving
	// the existing one, so when the display mode flips we have to
	// emit explicit delete-by-id escapes for the previous position.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	transition, _, _ := s.ConsumeGraphics("sidebar")
	if transition != "" {
		t.Errorf("first emit should not have a transition delete, got: %q", transition)
	}
	transition, _, _ = s.ConsumeGraphics("topbar")
	if !strings.Contains(transition, "a=d,d=A") {
		t.Errorf("anchor change should emit kitty delete-all (a=d,d=A); got: %q", transition)
	}
	transition, _, _ = s.ConsumeGraphics("topbar")
	if transition != "" {
		t.Errorf("same-anchor re-emit should not have a transition delete, got: %q", transition)
	}
}

func TestSidebar_HideGraphics_KittyEmitsDelete(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar") // mark as emitted

	hide := s.HideGraphics()
	if !strings.Contains(hide, "a=d,d=A") {
		t.Errorf("expected kitty delete-all escape (a=d,d=A), got: %q", hide)
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
	_, _, _ = s.ConsumeGraphics("sidebar") // mark emitted
	s.InvalidateGraphics()
	if got := s.HideGraphics(); got != "" {
		t.Errorf("expected HideGraphics after Invalidate to be no-op, got %d bytes", len(got))
	}
	if _, mm, _ := s.ConsumeGraphics("sidebar"); mm == "" {
		t.Errorf("expected ConsumeGraphics to keep returning escapes")
	}
}
