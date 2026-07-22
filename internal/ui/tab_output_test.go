package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestOutputPane_ScrollUp_ClampsInsteadOfLatching(t *testing.T) {
	p := NewOutputPane(1000)
	p.SetSize(80, 5)
	for i := 0; i < 20; i++ {
		p.Append(plainSegment(fmt.Sprintf("line %d", i)))
	}
	p.View() // populate the row cache

	p.ScrollUp(1000) // scroll far past the top
	p.View()

	// 20 rows, height 5 → maxScroll 15. The stored position must be clamped, not
	// latched at 1000 (which makes wheel-down appear dead for hundreds of ticks).
	if p.scrollPos != 15 {
		t.Fatalf("scrollPos after over-scroll = %d, want 15 (clamped, not latched)", p.scrollPos)
	}
	p.ScrollDown(1)
	if p.scrollPos != 14 {
		t.Fatalf("ScrollDown did not move immediately: scrollPos = %d, want 14", p.scrollPos)
	}
}

func TestOutputPane_StaysAnchoredAcrossTrims(t *testing.T) {
	p := NewOutputPane(30) // small cap so appends past cap trigger front-trims
	p.SetSize(80, 5)
	for i := 0; i < 30; i++ {
		p.Append(plainSegment(fmt.Sprintf("line %02d", i)))
	}
	p.View()       // rendered before the user scrolls
	p.ScrollUp(15) // view lines 10..14 (mid-buffer — these survive the trims below)
	before := p.View()

	// Each append trims the oldest line (00..09), none of which is in view; the
	// viewport must stay on lines 10..14, not drift toward the newest text.
	for i := 30; i < 40; i++ {
		p.Append(plainSegment(fmt.Sprintf("line %02d", i)))
	}
	after := p.View()

	if before != after {
		t.Errorf("viewport drifted across trims while scrolled up:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestOutputPane_ScrolledUp_StaysAnchoredOnAppend(t *testing.T) {
	p := NewOutputPane(1000) // large cap: no trimming, isolates anchoring
	p.SetSize(80, 5)
	for i := 0; i < 30; i++ {
		p.Append(plainSegment(fmt.Sprintf("line %d", i)))
	}
	p.View() // pane is rendered before the user scrolls (as it is every frame)
	p.ScrollUp(10)
	before := p.View()

	// Text arriving below the viewport must not slide it toward newer content —
	// you lose your place exactly when the game is noisy.
	for i := 30; i < 80; i++ {
		p.Append(plainSegment(fmt.Sprintf("line %d", i)))
	}
	after := p.View()

	if before != after {
		t.Errorf("viewport shifted while scrolled up during appends:\nbefore=%q\nafter=%q", before, after)
	}
}

func plainSegment(text string) []types.StyledSegment {
	return []types.StyledSegment{{Text: text}}
}

func TestOutputPane_AppendSuppressed_DefaultRendersPlaceholder(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.AppendSuppressed(plainSegment("[suppressed: Travis think]"), plainSegment("<Travis thinks aloud: hello>"))
	out := p.View()
	if !strings.Contains(out, "[suppressed: Travis think]") {
		t.Errorf("default View should show placeholder; got %q", out)
	}
	if strings.Contains(out, "thinks aloud") {
		t.Errorf("default View should NOT show original; got %q", out)
	}
}

func TestOutputPane_SetExpanded_RevealsOriginal(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.AppendSuppressed(plainSegment("[suppressed: Travis think]"), plainSegment("<Travis thinks aloud: hello>"))
	p.SetExpanded(true)
	out := p.View()
	if !strings.Contains(out, "thinks aloud") {
		t.Errorf("expanded View should show original; got %q", out)
	}
	if strings.Contains(out, "[suppressed:") {
		t.Errorf("expanded View should NOT show placeholder; got %q", out)
	}
}

func TestOutputPane_SetExpanded_TogglesBack(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.AppendSuppressed(plainSegment("[suppressed: Travis think]"), plainSegment("<Travis thinks aloud: hello>"))
	p.SetExpanded(true)
	p.SetExpanded(false)
	out := p.View()
	if !strings.Contains(out, "[suppressed: Travis think]") {
		t.Errorf("after toggle off, View should show placeholder again; got %q", out)
	}
}

func TestOutputPane_NormalLine_UnaffectedByExpand(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.Append(plainSegment("normal line"))
	p.SetExpanded(true)
	expandedView := p.View()
	p.SetExpanded(false)
	collapsedView := p.View()
	if !strings.Contains(expandedView, "normal line") || !strings.Contains(collapsedView, "normal line") {
		t.Error("normal lines should render the same regardless of expand state")
	}
}

func TestOutputPane_View_CachedAcrossNoOpCalls(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.Append(plainSegment("line 1"))
	p.Append(plainSegment("line 2"))

	v1 := p.View()
	v2 := p.View()
	if v1 != v2 {
		t.Fatal("View() should be byte-identical on repeat call with no state change")
	}
	if p.joinedCache == "" {
		t.Fatal("View should have populated joinedCache")
	}
}

func TestOutputPane_View_AppendInvalidatesJoinCache(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.Append(plainSegment("alpha"))
	_ = p.View()
	before := p.joinedCache

	p.Append(plainSegment("beta"))
	after := p.View()
	if before == after {
		t.Fatal("append should invalidate the joined cache and re-render")
	}
	if !strings.Contains(after, "beta") {
		t.Fatalf("post-append View should contain 'beta'; got %q", after)
	}
}

func TestOutputPane_View_ScrollInvalidatesJoinCache(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 3) // 3-row viewport
	for i := 0; i < 10; i++ {
		p.Append(plainSegment("line " + string(rune('A'+i))))
	}
	bottom := p.View()
	p.ScrollUp(2)
	scrolled := p.View()
	if bottom == scrolled {
		t.Fatal("scroll should change the joined output")
	}
}

func TestOutputPane_View_WidthChangeInvalidatesJoinCache(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.Append(plainSegment("hello world"))
	_ = p.View()
	if p.joinedCache == "" {
		t.Fatal("joinedCache should be populated after first View")
	}
	p.SetSize(40, 5)
	if p.joinedCache != "" {
		t.Fatal("width change should clear joinedCache")
	}
}

func TestOutputPane_View_ExpandToggleInvalidatesJoinCache(t *testing.T) {
	p := NewOutputPane(100)
	p.SetSize(80, 5)
	p.AppendSuppressed(plainSegment("[suppressed]"), plainSegment("original text"))
	_ = p.View()
	if p.joinedCache == "" {
		t.Fatal("joinedCache should be populated")
	}
	p.SetExpanded(true)
	if p.joinedCache != "" {
		t.Fatal("expand toggle should clear joinedCache")
	}
}

func TestOutputPane_AppendKeepsExistingMaxLinesBehavior(t *testing.T) {
	p := NewOutputPane(2) // tiny scrollback
	p.SetSize(80, 5)
	p.Append(plainSegment("line 1"))
	p.Append(plainSegment("line 2"))
	p.Append(plainSegment("line 3"))
	out := p.View()
	if strings.Contains(out, "line 1") {
		t.Error("oldest line should have been trimmed when scrollback overflowed")
	}
	if !strings.Contains(out, "line 2") || !strings.Contains(out, "line 3") {
		t.Errorf("expected line 2 and line 3 to remain, got %q", out)
	}
}

// BenchmarkOutputPane_FullRewrap measures a worst-case width-change re-wrap of a
// full 5000-line scrollback (the cost the resize-rewrap concern is about).
func BenchmarkOutputPane_FullRewrap(b *testing.B) {
	p := NewOutputPane(5000)
	p.SetSize(80, 40)
	for i := 0; i < 5000; i++ {
		p.Append(plainSegment("The quick brown fox jumps over the lazy dog and then keeps going a while."))
	}
	widths := []int{80, 79}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SetSize(widths[i%2], 40) // width change invalidates the row cache
		_ = p.View()               // forces a full re-wrap of all 5000 lines
	}
}
