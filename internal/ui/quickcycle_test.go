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

func TestQuickCycle_Next_FreshCycleStartsAtFirst(t *testing.T) {
	qc := NewQuickCycle([]string{"a", "b", "c"})
	got := []string{qc.Next(), qc.Next(), qc.Next(), qc.Next()}
	want := []string{"a", "b", "c", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cycle = %v, want %v (first Alt+M must not skip the first mode)", got, want)
		}
	}
}

func TestQuickCycle_Next_SingleEntryCyclesOntoItself(t *testing.T) {
	qc := NewQuickCycle([]string{"solo"})
	if a, b := qc.Next(), qc.Next(); a != "solo" || b != "solo" {
		t.Fatalf("single-entry cycle = %q,%q, want solo,solo", a, b)
	}
}
