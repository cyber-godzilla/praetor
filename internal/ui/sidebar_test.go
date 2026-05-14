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

func TestSidebar_Kitty_ThrottlesRepeatEmissions(t *testing.T) {
	// kitty path now skips re-emission on frames where nothing changed
	// and we're still within the self-heal interval. The image stays
	// on screen via the previous placement; emitting again every frame
	// only accumulates parser state in some terminals (input lag).
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)

	_, mm1, _ := s.ConsumeGraphics("sidebar")
	if mm1 == "" {
		t.Fatal("first ConsumeGraphics must return the minimap escape")
	}
	_, mm2, cp2 := s.ConsumeGraphics("sidebar")
	if mm2 != "" || cp2 != "" {
		t.Errorf("second call within self-heal interval should return empty; got mm=%d bytes cp=%d bytes", len(mm2), len(cp2))
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

func TestSidebar_View_CachedAcrossNoOpFrames(t *testing.T) {
	// View() should return the byte-identical cached string on repeat
	// calls when no state has changed. This is the hot path: bubbletea
	// re-runs View() on every cursor blink / event tick.
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	s.UpdateVitals(intp(80), intp(80), intp(20), intp(80))
	s.UpdateLighting(types.LightBright, 40)

	v1 := s.View()
	v2 := s.View()
	if v1 != v2 {
		t.Fatalf("View() should be byte-identical on repeat calls when nothing changed")
	}
	// Hold onto the pointer-ish identity: setting a value to the same
	// thing it already is must not invalidate the cache.
	s.UpdateVitals(intp(80), intp(80), intp(20), intp(80))
	if v3 := s.View(); v3 != v1 {
		t.Fatalf("UpdateVitals with identical values should not invalidate cache")
	}
}

func TestSidebar_View_VitalsChangeInvalidates(t *testing.T) {
	// Note: in go test there's no TTY, so lipgloss strips color escapes
	// — different bar colors produce identical whitespace. We assert
	// the dirty flag directly instead of a textual diff.
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	s.UpdateVitals(intp(100), intp(100), intp(0), intp(100))
	_ = s.View() // prime the cache, clears viewDirty

	if s.viewDirty {
		t.Fatalf("viewDirty should be false after View() with no pending changes")
	}
	s.UpdateVitals(intp(25), nil, nil, nil)
	if !s.viewDirty {
		t.Fatalf("vitals change should mark view cache dirty")
	}

	// Updating with the same value should NOT re-dirty.
	_ = s.View()
	s.UpdateVitals(intp(25), nil, nil, nil)
	if s.viewDirty {
		t.Fatalf("identical vitals update should not mark dirty")
	}
}

func TestSidebar_View_LightingChangeInvalidates(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	s.UpdateLighting(types.LightBright, 40)
	before := s.View()

	s.UpdateLighting(types.LightVeryDark, 5)
	after := s.View()
	if before == after {
		t.Fatalf("lighting change should invalidate the view cache")
	}
}

func TestSidebar_View_MinimapUpdateInvalidates(t *testing.T) {
	// UpdateMinimap marks graphicsDirty; the next View call rebuilds the
	// graphics cache, which feeds new placeholders into the text. We
	// assert via the dirty flag because in ModeNone the placeholder
	// fallback ("Minimap unavailable") is the same string before/after.
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	_ = s.View() // prime cache

	if s.viewDirty {
		t.Fatalf("viewDirty should be false after View() with no pending changes")
	}
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	if !s.graphicsDirty {
		t.Fatalf("UpdateMinimap should mark graphicsDirty")
	}
	_ = s.View() // graphicsDirty triggers rebuildGraphicsCache → viewDirty=true → rebuild → false
	if s.viewDirty || s.graphicsDirty {
		t.Fatalf("View() should clear both dirty flags; got viewDirty=%v graphicsDirty=%v", s.viewDirty, s.graphicsDirty)
	}
}

func TestSidebar_View_ResizeInvalidates(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	before := s.View()

	s.SetSize(60, 30)
	after := s.View()
	if before == after {
		t.Fatalf("resize should invalidate the view cache")
	}
}

func TestSidebar_View_CompactToggleInvalidates(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeNone)
	s.SetSize(40, 30)
	before := s.View()

	s.SetCompact(true)
	after := s.View()
	if before == after {
		t.Fatalf("compact toggle should invalidate the view cache")
	}
}

func intp(i int) *int { return &i }

func TestSidebar_Kitty_SelfHealEmitsAfterInterval(t *testing.T) {
	// Even when nothing changes, ConsumeGraphics must re-emit the cached
	// escape after kittySelfHealInterval frames so a terminal that
	// silently dropped the image picks it back up within ~1 second.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10, Color: "#ff0000", Brightness: 25}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar")

	// Jump the counter to the threshold; the next call must re-emit.
	s.kittyFramesSinceEmit = kittySelfHealInterval
	_, mm, _ := s.ConsumeGraphics("sidebar")
	if mm == "" {
		t.Fatal("self-heal interval reached but ConsumeGraphics returned no minimap escape")
	}
	if s.kittyFramesSinceEmit != 0 {
		t.Fatalf("counter should reset to 0 after self-heal emit; got %d", s.kittyFramesSinceEmit)
	}
}

func TestSidebar_Kitty_DirtyDataEmitsImmediately(t *testing.T) {
	// UpdateMinimap (or any setter that marks dirtySinceEmit) must
	// bypass the throttle so room changes are visible right away.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0, Size: 10}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar")

	// Throttle is engaged: counter at 1, no change since last emit.
	_, mm1, _ := s.ConsumeGraphics("sidebar")
	if mm1 != "" {
		t.Fatalf("expected throttled call to return empty; got %d bytes", len(mm1))
	}

	// Data changes — next call must emit even though we're well under
	// the self-heal interval.
	s.UpdateMinimap([]types.MinimapRoom{{X: 5, Y: 5, Size: 10}}, nil)
	_, mm2, _ := s.ConsumeGraphics("sidebar")
	if mm2 == "" {
		t.Fatal("dirty minimap data should emit immediately, regardless of throttle")
	}
}

func TestSidebar_Kitty_AnchorChangeEmitsImmediately(t *testing.T) {
	// The throttle must not delay anchor-change emissions — the image
	// has to move (delete at old position, re-emit at new) right away.
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar")
	// Engage the throttle.
	_, _, _ = s.ConsumeGraphics("sidebar")

	transition, mm, _ := s.ConsumeGraphics("topbar")
	if !strings.Contains(transition, "a=d,d=A") {
		t.Errorf("anchor change should emit delete-all transition; got %q", transition)
	}
	if mm == "" {
		t.Error("anchor change should emit the new-position image, not be throttled")
	}
}

func TestSidebar_Kitty_HideResetsThrottleCounter(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar")
	s.kittyFramesSinceEmit = 30

	_ = s.HideGraphics()
	if s.kittyFramesSinceEmit != 0 {
		t.Fatalf("HideGraphics should reset throttle counter; got %d", s.kittyFramesSinceEmit)
	}
}

func TestSidebar_Kitty_InvalidateResetsThrottleCounter(t *testing.T) {
	s := NewSidebar(0.8, 12, graphics.ModeKitty)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0}}, nil)
	_, _, _ = s.ConsumeGraphics("sidebar")
	s.kittyFramesSinceEmit = 30

	s.InvalidateGraphics()
	if s.kittyFramesSinceEmit != 0 {
		t.Fatalf("InvalidateGraphics should reset throttle counter; got %d", s.kittyFramesSinceEmit)
	}
}

func TestSidebar_Sixel_NotThrottled(t *testing.T) {
	// Sixel pixels live in the cell buffer and get overwritten by any
	// surrounding text write, so unlike kitty the sixel path must emit
	// every frame — no throttle for sixel.
	s := NewSidebar(0.8, 12, graphics.ModeSixel)
	s.SetSize(40, 30)
	s.UpdateMinimap([]types.MinimapRoom{{X: 0, Y: 0}}, nil)
	_, mm1, _ := s.ConsumeGraphics("sidebar")
	if mm1 == "" {
		t.Fatal("first sixel emit should return escape")
	}
	_, mm2, _ := s.ConsumeGraphics("sidebar")
	if mm2 != mm1 {
		t.Errorf("sixel must emit on every call; got mm2=%d bytes (mm1=%d)", len(mm2), len(mm1))
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
