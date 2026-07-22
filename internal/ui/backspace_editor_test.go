package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Editor-level regression for the multibyte-backspace fix: erasing a non-ASCII
// character must remove the whole rune, not one byte (which left invalid UTF-8).
func TestHighlightsEditor_BackspaceOverMultibyte(t *testing.T) {
	hm := NewHighlightsManager(nil)
	hm.editing = true
	hm.editBuf = "café"

	hm, _ = hm.updateEditing(tea.KeyMsg{Type: tea.KeyBackspace})

	if hm.editBuf != "caf" {
		t.Errorf("editBuf after backspace = %q, want %q", hm.editBuf, "caf")
	}
}
