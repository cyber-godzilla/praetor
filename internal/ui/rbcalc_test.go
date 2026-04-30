package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/calc"
)

func newCalcScreen() RBCalcScreen {
	s := NewRBCalcScreen()
	s.SetSize(120, 40)
	return s
}

func TestRBCalc_InitialState(t *testing.T) {
	s := newCalcScreen()
	if s.fieldFocus != 0 {
		t.Errorf("initial fieldFocus = %d, want 0", s.fieldFocus)
	}
	for i, b := range s.fieldBufs {
		if b != "" {
			t.Errorf("fieldBufs[%d] = %q, want empty", i, b)
		}
	}
	if s.mode != calc.ModeDefensive {
		t.Errorf("initial mode = %v, want ModeDefensive", s.mode)
	}
	if s.selfTrained || s.selfTaught {
		t.Errorf("toggles should default off; got selfTrained=%v selfTaught=%v", s.selfTrained, s.selfTaught)
	}
}

func TestRBCalc_TabAdvancesFocus(t *testing.T) {
	s := newCalcScreen()
	for want := 1; want <= 4; want++ {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyTab})
		expect := want % 4
		if s.fieldFocus != expect {
			t.Errorf("after %d Tabs fieldFocus = %d, want %d", want, s.fieldFocus, expect)
		}
	}
}

func TestRBCalc_ShiftTabRetreatsFocus(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if s.fieldFocus != 3 {
		t.Errorf("shift+tab from 0 should wrap to 3, got %d", s.fieldFocus)
	}
}

func TestRBCalc_DigitsAppendToFocusedField(t *testing.T) {
	s := newCalcScreen()
	for _, r := range "1150" {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if s.fieldBufs[0] != "1150" {
		t.Errorf("fieldBufs[0] = %q, want %q", s.fieldBufs[0], "1150")
	}
}

func TestRBCalc_NonDigitRunesIgnored(t *testing.T) {
	s := newCalcScreen()
	// Letters that are not mode/toggle hotkeys
	for _, r := range "abcfg" {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if s.fieldBufs[0] != "" {
		t.Errorf("non-digit non-hotkey runes should not append; got %q", s.fieldBufs[0])
	}
}

func TestRBCalc_BackspaceDeletes(t *testing.T) {
	s := newCalcScreen()
	for _, r := range "12345" {
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if s.fieldBufs[0] != "123" {
		t.Errorf("after 2 backspaces, fieldBufs[0] = %q, want %q", s.fieldBufs[0], "123")
	}
}

func TestRBCalc_OToggleMode(t *testing.T) {
	s := newCalcScreen()
	for _, r := range "OdN" {
		want := map[rune]calc.Mode{'O': calc.ModeOffensive, 'd': calc.ModeDefensive, 'N': calc.ModeNoncombat}[r]
		s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if s.mode != want {
			t.Errorf("after key %q, mode = %v, want %v", r, s.mode, want)
		}
	}
}

func TestRBCalc_TToggleSelfTrained(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	if !s.selfTrained {
		t.Error("T should turn selfTrained ON")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if s.selfTrained {
		t.Error("t should turn selfTrained back OFF")
	}
}

func TestRBCalc_HToggleSelfTaught(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if !s.selfTaught {
		t.Error("H should turn selfTaught ON")
	}
}

func TestRBCalc_EscEmitsCloseMsg(t *testing.T) {
	s := newCalcScreen()
	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should produce a close command")
	}
	msg := cmd()
	if _, ok := msg.(RBCalcCloseMsg); !ok {
		t.Errorf("Esc should emit RBCalcCloseMsg, got %T", msg)
	}
}
