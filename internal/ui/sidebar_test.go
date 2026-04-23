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

	mm, cp := s.GraphicsEscapes()
	if mm != "" || cp != "" {
		t.Errorf("expected empty escapes in ModeNone, got mm=%d bytes cp=%d bytes", len(mm), len(cp))
	}

	view := s.View()
	if !strings.Contains(view, "Minimap unavailable") {
		t.Errorf("expected sidebar view to contain fallback text, got:\n%s", view)
	}
}
