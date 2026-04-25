package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModePicker_EnterOnEmptyDoesNotPanic(t *testing.T) {
	mp := NewModePicker(nil, nil)
	// Should not panic.
	_, _ = mp.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestModePicker_SpaceOnEmptyDoesNotPanic(t *testing.T) {
	mp := NewModePicker(nil, nil)
	// Should not panic.
	_, _ = mp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
}

func TestModePicker_EnterTogglesWhenPopulated(t *testing.T) {
	mp := NewModePicker([]string{"alpha", "beta"}, nil)
	mp, _ = mp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !mp.selected["alpha"] {
		t.Errorf("expected alpha selected after Enter, got %v", mp.selected)
	}
}
