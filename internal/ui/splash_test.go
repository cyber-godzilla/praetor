package ui

import (
	"strings"
	"testing"
)

// TestCenterVersion guards the splash version field. Regression: "v0.2.10" (7
// chars, the first two-digit patch number) was truncated to "v0.2.1" by a fixed
// 6-char slot.
func TestCenterVersion(t *testing.T) {
	got := centerVersion("v0.2.10", splashBoxInterior)
	if len(got) != splashBoxInterior {
		t.Errorf("width = %d, want %d", len(got), splashBoxInterior)
	}
	if strings.TrimSpace(got) != "v0.2.10" {
		t.Errorf("trimmed = %q, want v0.2.10 (no truncation)", strings.TrimSpace(got))
	}

	// Previous ≤6-char versions still fill the field exactly.
	for _, v := range []string{"dev", "v0.2.9", ""} {
		g := centerVersion(v, splashBoxInterior)
		if len(g) != splashBoxInterior || strings.TrimSpace(g) != strings.TrimSpace(v) {
			t.Errorf("centerVersion(%q) = %q (len %d)", v, strings.TrimSpace(g), len(g))
		}
	}

	// An absurdly long version is clipped so the box border can't overflow.
	if l := len(centerVersion(strings.Repeat("x", 100), splashBoxInterior)); l != splashBoxInterior {
		t.Errorf("overlong width = %d, want %d", l, splashBoxInterior)
	}
}
