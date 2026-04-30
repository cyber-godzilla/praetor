package ui

import (
	"strings"
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

func TestRBCalc_View_NonEmpty(t *testing.T) {
	s := newCalcScreen()
	if s.View() == "" {
		t.Fatal("View should be non-empty even with no inputs")
	}
}

func TestRBCalc_View_RendersInputLabels(t *testing.T) {
	s := newCalcScreen()
	view := s.View()
	for _, lbl := range []string{"Current:", "Target:", "Basics", "Subskill"} {
		if !strings.Contains(view, lbl) {
			t.Errorf("View missing label %q", lbl)
		}
	}
}

func TestRBCalc_View_DefensiveScreenshotCase(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "1150"
	s.fieldBufs[1] = "500"
	view := s.View()
	// The Def. row Easy column should show 280 (canonical screenshot value).
	if !strings.Contains(view, "280") {
		t.Errorf("View missing canonical Def. Easy = 280; got:\n%s", view)
	}
	// Impossible row Aggressive column = 42.7
	if !strings.Contains(view, "42.7") {
		t.Errorf("View missing canonical Aggr. Impos. = 42.7")
	}
}

func TestRBCalc_View_NoncombatModeShowsSingleRow(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "1150"
	s.fieldBufs[1] = "500"
	s.mode = calc.ModeNoncombat
	view := s.View()
	if !strings.Contains(view, "280") {
		t.Errorf("Noncombat Easy should show 280; got:\n%s", view)
	}
	// Noncombat shouldn't render posture row labels.
	if strings.Contains(view, "Bers.") {
		t.Errorf("Noncombat view should not show 'Bers.' row label")
	}
}

func TestRBCalc_View_NoTargetHidesComparisonAndCost(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "1150"
	s.fieldBufs[1] = "500"
	view := s.View()
	if strings.Contains(view, "Training cost") {
		t.Error("Training cost panel should be hidden without target ranks")
	}
	if strings.Count(view, "Easy") > 1 {
		t.Errorf("only the current table should be visible; got %d 'Easy' columns", strings.Count(view, "Easy"))
	}
}

func TestRBCalc_View_TargetShowsSecondTable(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "1150"
	s.fieldBufs[1] = "500"
	s.fieldBufs[2] = "2000"
	s.fieldBufs[3] = "850"
	view := s.View()
	if strings.Count(view, "Basic") < 2 {
		t.Errorf("two RB tables should each have a 'Basic' column; got %d occurrences", strings.Count(view, "Basic"))
	}
	if !strings.Contains(view, "Target") {
		t.Error("target table label missing")
	}
}

func TestRBCalc_View_TrainingCostShowsTogglesAndTable(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "1150"
	s.fieldBufs[1] = "500"
	s.fieldBufs[2] = "2000"
	s.fieldBufs[3] = "850"
	view := s.View()
	for _, want := range []string{
		"Training cost",
		"ΔBasics",
		"ΔSubskill",
		"Self-Trained",
		"Using selftrain command",
		"Self-Taught",
		"Has self-taught trait",
		"Skill Point Cost to Train",
		"1st",
		"20th",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("training panel missing %q", want)
		}
	}
}

func TestRBCalc_View_SelfTrainedDoublesBaseLayerCost(t *testing.T) {
	// At slot 1 easy training 0->1, cost is 10 (selfTrained off) and
	// 20 (selfTrained on). Verify both numbers appear depending on
	// the toggle state.
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "0"
	s.fieldBufs[2] = "0"
	s.fieldBufs[3] = "1"
	off := s.View()
	if !strings.Contains(off, " 10 ") {
		t.Errorf("selfTrained-off slot 1 easy 0->1 should show 10; got:\n%s", off)
	}
	s.selfTrained = true
	on := s.View()
	if !strings.Contains(on, " 20 ") {
		t.Errorf("selfTrained-on slot 1 easy 0->1 should show 20; got:\n%s", on)
	}
}
