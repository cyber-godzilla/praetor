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
	if s.selfTrained || s.selfTaught || s.healing {
		t.Errorf("toggles should default off; got selfTrained=%v selfTaught=%v healing=%v",
			s.selfTrained, s.selfTaught, s.healing)
	}
	if s.slotPage != 0 {
		t.Errorf("initial slotPage = %d, want 0", s.slotPage)
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

func TestRBCalc_LToggleSelfTaught(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	if !s.selfTaught {
		t.Error("L should turn selfTaught ON")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if s.selfTaught {
		t.Error("l should turn selfTaught back OFF")
	}
}

func TestRBCalc_HToggleHealing(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	if !s.healing {
		t.Error("H should turn healing ON")
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if s.healing {
		t.Error("h should turn healing back OFF")
	}
}

func TestRBCalc_STogglesSlotPage(t *testing.T) {
	s := newCalcScreen()
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	if s.slotPage != 1 {
		t.Errorf("S should flip slotPage to 1, got %d", s.slotPage)
	}
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if s.slotPage != 0 {
		t.Errorf("s should flip slotPage back to 0, got %d", s.slotPage)
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
		"Healing",
		"Healing-typed skill (+5 SP/rank)",
		"Skill Point Cost to Train",
		"Slots 1-10",
		"1st",
		"10th",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("training panel missing %q", want)
		}
	}
}

func TestRBCalc_View_BasicsSPShownInline(t *testing.T) {
	// curBasics=0, tgtBasics=1, selfTrained/selfTaught/healing all off:
	// basics SP = TrainSPCost(0, 1, 1, Easy, ...) = 10. Verify it
	// surfaces on the ΔBasics line.
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "100"
	s.fieldBufs[2] = "1"
	s.fieldBufs[3] = "200"
	view := s.View()
	if !strings.Contains(view, "ΔBasics: +1 (10 SP)") {
		t.Errorf("expected ΔBasics line to include '(10 SP)'; got:\n%s", view)
	}
}

func TestRBCalc_View_BasicsSPHiddenWhenNoBasicsDelta(t *testing.T) {
	// No basics increase → don't render the parenthetical SP cost.
	s := newCalcScreen()
	s.fieldBufs[0] = "100"
	s.fieldBufs[1] = "100"
	s.fieldBufs[2] = "100"
	s.fieldBufs[3] = "200"
	view := s.View()
	if strings.Contains(view, "(0 SP)") {
		t.Errorf("ΔBasics with zero delta should not render '(0 SP)'; got:\n%s", view)
	}
}

func TestRBCalc_View_SlotPagePaginates(t *testing.T) {
	// Default page (0) shows slots 1-10 only; page 1 shows slots 11-20.
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "0"
	s.fieldBufs[2] = "0"
	s.fieldBufs[3] = "10"

	page0 := s.View()
	if !strings.Contains(page0, "1st") || !strings.Contains(page0, "10th") {
		t.Errorf("page 0 should show 1st and 10th; got:\n%s", page0)
	}
	if strings.Contains(page0, "11th") || strings.Contains(page0, "20th") {
		t.Errorf("page 0 should NOT show 11th or 20th; got:\n%s", page0)
	}
	if !strings.Contains(page0, "Slots 1-10") {
		t.Errorf("page 0 should label 'Slots 1-10'")
	}

	s.slotPage = 1
	page1 := s.View()
	if !strings.Contains(page1, "11th") || !strings.Contains(page1, "20th") {
		t.Errorf("page 1 should show 11th and 20th; got:\n%s", page1)
	}
	if strings.Contains(page1, " 1st") || strings.Contains(page1, "10th\n") {
		// (looser checks: 1st could appear elsewhere; rely on row labels)
		t.Errorf("page 1 should NOT include 1st-10th rows; got:\n%s", page1)
	}
	if !strings.Contains(page1, "Slots 11-20") {
		t.Errorf("page 1 should label 'Slots 11-20'")
	}
}

func TestRBCalc_View_HealingAdds5SPPerRank(t *testing.T) {
	// 0->1 easy at slot 1: cost 10 normally, 15 with healing (+5).
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "0"
	s.fieldBufs[2] = "0"
	s.fieldBufs[3] = "1"
	off := s.View()
	if !strings.Contains(off, " 10 ") {
		t.Errorf("healing-off should show 10; got:\n%s", off)
	}
	s.healing = true
	on := s.View()
	if !strings.Contains(on, " 15 ") {
		t.Errorf("healing-on should show 15 (10 + 5); got:\n%s", on)
	}
}

func TestRBCalc_View_OverThresholdWarning(t *testing.T) {
	// tgtSub > 1150 with selfTrained=false → green warning visible.
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "0"
	s.fieldBufs[2] = "0"
	s.fieldBufs[3] = "1200"
	view := s.View()
	if !strings.Contains(view, "ranks 1151+ require /selftrain") {
		t.Errorf("expected 1151+ warning when target > 1150 and selfTrained off; got:\n%s", view)
	}

	// With selfTrained ON, no warning needed.
	s.selfTrained = true
	if got := s.View(); strings.Contains(got, "1151+ require") {
		t.Errorf("warning should be hidden when selfTrained=true; got:\n%s", got)
	}
}

func TestRBCalc_View_NoWarningBelowThreshold(t *testing.T) {
	s := newCalcScreen()
	s.fieldBufs[0] = "0"
	s.fieldBufs[1] = "0"
	s.fieldBufs[2] = "0"
	s.fieldBufs[3] = "1150"
	if got := s.View(); strings.Contains(got, "1151+ require") {
		t.Errorf("warning should be hidden at exactly 1150; got:\n%s", got)
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
