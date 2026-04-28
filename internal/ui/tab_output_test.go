package ui

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

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
