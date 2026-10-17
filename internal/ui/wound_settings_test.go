package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestWoundSettingsScreenEditsAndReportsChanges(t *testing.T) {
	initial := config.WoundHighlightConfig{
		Cuts: "red", Bruises: "purple", Punctures: "blue", Burns: "orange",
	}
	screen := NewWoundSettingsScreen(initial)

	// Toggle the opt-in setting, move to Cuts, and cycle its family.
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyRight})

	if !screen.cfg.Enabled || screen.cfg.Cuts != "purple" {
		t.Fatalf("edited wound settings = %+v", screen.cfg)
	}
	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := cmd().(WoundSettingsCloseMsg)
	if !msg.Changed || msg.Config != screen.cfg {
		t.Fatalf("close message = %+v, want changed config %+v", msg, screen.cfg)
	}
}

func TestWoundSettingsScreenWrapsFamiliesBackward(t *testing.T) {
	screen := NewWoundSettingsScreen(config.WoundHighlightConfig{
		Cuts: "red", Bruises: "purple", Punctures: "blue", Burns: "orange",
	})
	screen.cursor = 1
	screen, _ = screen.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if screen.cfg.Cuts != "orange" {
		t.Fatalf("backward family wrap = %q, want orange", screen.cfg.Cuts)
	}
}

func TestWoundSettingsViewNamesAllControls(t *testing.T) {
	screen := NewWoundSettingsScreen(config.WoundHighlightConfig{
		Cuts: "red", Bruises: "purple", Punctures: "blue", Burns: "orange",
	})
	view := screen.View()
	for _, label := range []string{"Highlight wounds", "Cuts", "Bruises / fractures", "Punctures / wounds", "Burns"} {
		if !strings.Contains(view, label) {
			t.Errorf("view missing %q: %s", label, view)
		}
	}
}
