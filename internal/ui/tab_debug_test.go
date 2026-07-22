package ui

import "testing"

func TestDebugPane_ScrollDown_ClampsToContent(t *testing.T) {
	d := NewDebugPane()
	d.SetSize(80, 5)

	d.ScrollDown(1000) // far past the content

	max := d.debugMaxScroll()
	if d.scroll != max {
		t.Fatalf("scroll = %d after over-scroll, want clamped to %d", d.scroll, max)
	}
	if d.scroll >= 1000 {
		t.Fatalf("scroll latched at %d instead of clamping", d.scroll)
	}

	// Scrolling back up moves immediately rather than burning ticks on the latch.
	d.ScrollUp(1)
	if d.scroll != max-1 && max > 0 {
		t.Fatalf("ScrollUp did not move: scroll = %d, want %d", d.scroll, max-1)
	}
}
