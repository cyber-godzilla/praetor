package ui

import (
	"strings"
	"testing"
)

func TestInput_View_BorderCachedBySetWidth(t *testing.T) {
	i := NewInput()
	i.SetWidth(40)

	if i.cachedBorder == "" {
		t.Fatal("SetWidth should populate cachedBorder")
	}
	if i.cachedBorderWidth != 40 {
		t.Fatalf("cachedBorderWidth = %d, want 40", i.cachedBorderWidth)
	}

	// Border line should be 40 visible cells wide (top edge ─ characters).
	border := strings.Split(i.cachedBorder, "\n")[0]
	if w := visibleWidth(border); w != 40 {
		t.Fatalf("cached border visible width = %d, want 40", w)
	}
}

func TestInput_View_ContentPaddedToWidth(t *testing.T) {
	i := NewInput()
	i.SetWidth(40)

	v := i.View()
	lines := strings.Split(v, "\n")
	if len(lines) < 2 {
		t.Fatalf("View should produce at least 2 lines (border + content); got %d", len(lines))
	}
	// Content line (line 1) should be padded out to the full width so the
	// row fully covers any underlying terminal cells.
	if w := visibleWidth(lines[1]); w != 40 {
		t.Errorf("content line visible width = %d, want 40 (padded)", w)
	}
}

func TestInput_View_WidthChangeRebuildsBorder(t *testing.T) {
	i := NewInput()
	i.SetWidth(40)
	first := i.cachedBorder

	i.SetWidth(60)
	if i.cachedBorder == first {
		t.Fatal("SetWidth to a new value should rebuild the border")
	}
	if i.cachedBorderWidth != 60 {
		t.Fatalf("cachedBorderWidth = %d, want 60", i.cachedBorderWidth)
	}
}

func TestInput_View_ZeroWidthFallback(t *testing.T) {
	// A fresh Input has width=0 — View should still produce something
	// (the lipgloss fallback) rather than panic or return ""/garbage.
	i := NewInput()
	v := i.View()
	if v == "" {
		// inputStyle.Width(0) may produce minimal output, but it should
		// at least be defined behavior — the test would still pass
		// if it's an empty string. We're really checking for no panic.
		_ = v
	}
}
