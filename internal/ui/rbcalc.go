package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/calc"
)

// MenuRBCalcMsg opens the calculator screen.
type MenuRBCalcMsg struct{}

// RBCalcCloseMsg dismisses the calculator and returns to game state.
type RBCalcCloseMsg struct{}

// RBCalcScreen is the modal rank-bonus + training-cost calculator.
type RBCalcScreen struct {
	width, height int
	fieldFocus    int       // 0..3 = curBasics, curSub, tgtBasics, tgtSub
	fieldBufs     [4]string // raw text for each numeric field
	mode          calc.Mode
	selfTrained   bool
	selfTaught    bool
}

// NewRBCalcScreen returns a fresh calculator with empty inputs and
// defensive mode selected.
func NewRBCalcScreen() RBCalcScreen {
	return RBCalcScreen{mode: calc.ModeDefensive}
}

func (s *RBCalcScreen) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// Update handles a key event. Returns a possibly-mutated copy of the
// screen and an optional command (only emitted on Esc).
func (s RBCalcScreen) Update(msg tea.KeyMsg) (RBCalcScreen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return s, func() tea.Msg { return RBCalcCloseMsg{} }
	case tea.KeyTab:
		s.fieldFocus = (s.fieldFocus + 1) % 4
		return s, nil
	case tea.KeyShiftTab:
		s.fieldFocus = (s.fieldFocus + 3) % 4
		return s, nil
	case tea.KeyBackspace:
		buf := s.fieldBufs[s.fieldFocus]
		if len(buf) > 0 {
			s.fieldBufs[s.fieldFocus] = buf[:len(buf)-1]
		}
		return s, nil
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return s, nil
		}
		r := msg.Runes[0]
		// Mode hotkeys (any case).
		switch r {
		case 'O', 'o':
			s.mode = calc.ModeOffensive
			return s, nil
		case 'D', 'd':
			s.mode = calc.ModeDefensive
			return s, nil
		case 'N', 'n':
			s.mode = calc.ModeNoncombat
			return s, nil
		case 'T', 't':
			s.selfTrained = !s.selfTrained
			return s, nil
		case 'H', 'h':
			s.selfTaught = !s.selfTaught
			return s, nil
		}
		// Numeric input.
		if r >= '0' && r <= '9' {
			s.fieldBufs[s.fieldFocus] += string(r)
		}
		return s, nil
	}
	return s, nil
}

// View is implemented in a later task.
func (s RBCalcScreen) View() string {
	return ""
}
