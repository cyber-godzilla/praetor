package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newOOCScreen(initial []string) IgnorelistScreen {
	s := NewIgnorelistScreen(IgnorelistKindOOC, initial)
	s.SetSize(80, 30)
	return s
}

func TestIgnorelistScreen_StartsEmpty(t *testing.T) {
	s := newOOCScreen(nil)
	if len(s.names) != 0 {
		t.Errorf("expected empty names, got %v", s.names)
	}
}

func TestIgnorelistScreen_AddFlow(t *testing.T) {
	s := newOOCScreen(nil)
	// Press 'A' to enter add mode.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !s.adding {
		t.Fatal("expected adding=true after 'A'")
	}
	// Type "xXSephirothXx" then Enter.
	for _, r := range "xXSephirothXx" {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s.adding {
		t.Error("expected adding=false after Enter")
	}
	if len(s.names) != 1 || s.names[0] != "xXSephirothXx" {
		t.Errorf("expected names=[xXSephirothXx], got %v", s.names)
	}
}

func TestIgnorelistScreen_AddDuplicateIsNoop(t *testing.T) {
	s := newOOCScreen([]string{"dArKwInG666"})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	for _, r := range "DARKWING666" { // case-insensitive duplicate
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(s.names) != 1 {
		t.Errorf("expected dedupe to keep one entry, got %v", s.names)
	}
}

func TestIgnorelistScreen_RemoveFlow(t *testing.T) {
	s := newOOCScreen([]string{"M0rt1c1aNvOiD", "EmoCryBaby"})
	// Cursor on 0. Press 'D' then 'Y'.
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !s.confirm {
		t.Fatal("expected confirm=true after 'D'")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if s.confirm {
		t.Error("expected confirm=false after 'Y'")
	}
	if len(s.names) != 1 || s.names[0] != "EmoCryBaby" {
		t.Errorf("expected names=[EmoCryBaby], got %v", s.names)
	}
}

func TestIgnorelistScreen_EscEmitsOOCCloseMsg(t *testing.T) {
	s := newOOCScreen([]string{"MasterChief1337"})
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should produce a close command")
	}
	msg := cmd()
	closeMsg, ok := msg.(IgnorelistOOCCloseMsg)
	if !ok {
		t.Fatalf("expected IgnorelistOOCCloseMsg, got %T", msg)
	}
	if !closeMsg.Changed {
		t.Error("expected Changed=true on close")
	}
	if len(closeMsg.Names) != 1 || closeMsg.Names[0] != "MasterChief1337" {
		t.Errorf("expected names=[MasterChief1337], got %v", closeMsg.Names)
	}
}

func TestIgnorelistScreen_EscEmitsThinkCloseMsg(t *testing.T) {
	s := NewIgnorelistScreen(IgnorelistKindThink, []string{"Travis", "Andrea"})
	s.SetSize(80, 30)
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	msg := cmd()
	if _, ok := msg.(IgnorelistThinkCloseMsg); !ok {
		t.Fatalf("expected IgnorelistThinkCloseMsg, got %T", msg)
	}
}

func TestIgnorelistScreen_ViewRendersWithoutPanic(t *testing.T) {
	s := NewIgnorelistScreen(IgnorelistKindOOC, []string{"xXSephirothXx"})
	s.SetSize(80, 30)
	if got := s.View(); got == "" {
		t.Error("View returned empty string")
	}
	s2 := NewIgnorelistScreen(IgnorelistKindThink, nil) // empty
	s2.SetSize(80, 30)
	if got := s2.View(); got == "" {
		t.Error("empty-state View returned empty string")
	}
}
