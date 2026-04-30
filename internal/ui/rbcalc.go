package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// View renders the calculator UI.
func (s RBCalcScreen) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	boxWidth := s.width - 4
	if boxWidth < 60 {
		boxWidth = 60
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 2).
		Width(boxWidth)

	curBasics, curSub, _, _ := s.parsedInputs()

	var b strings.Builder
	b.WriteString(titleStyle.Render("Rank Bonus Calculator"))
	b.WriteString("\n\n")

	// Input header.
	b.WriteString(s.renderInputHeader())
	b.WriteString("\n\n")

	// Current RB table.
	b.WriteString(s.renderRBTable("Current", curBasics, curSub))
	b.WriteString("\n")

	// Footer hint.
	b.WriteString(dimStyle.Render("[Tab] field   [O/D/N] mode   [T] Self-Trained   [H] Self-Taught   [Esc] close"))

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}

// parsedInputs decodes the four field buffers into ints. Empty/invalid
// fields parse to 0.
func (s RBCalcScreen) parsedInputs() (curBasics, curSub, tgtBasics, tgtSub int) {
	parse := func(buf string) int {
		n, _ := strconv.Atoi(strings.TrimSpace(buf))
		if n < 0 {
			return 0
		}
		return n
	}
	return parse(s.fieldBufs[0]), parse(s.fieldBufs[1]),
		parse(s.fieldBufs[2]), parse(s.fieldBufs[3])
}

// renderInputHeader draws the four-field input table with focus markers
// and the current mode label on the right.
func (s RBCalcScreen) renderInputHeader() string {
	cellStyle := lipgloss.NewStyle().Foreground(colorDim).Bold(true)
	focusStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	field := func(idx int, label string) string {
		buf := s.fieldBufs[idx]
		if buf == "" {
			buf = "____"
		}
		text := fmt.Sprintf(" %s [%s] ", label, buf)
		if s.fieldFocus == idx {
			return focusStyle.Render(text)
		}
		return cellStyle.Render(text)
	}
	modeName := map[calc.Mode]string{
		calc.ModeDefensive: "Defensive",
		calc.ModeOffensive: "Offensive",
		calc.ModeNoncombat: "Noncombat",
	}[s.mode]
	mode := lipgloss.NewStyle().Foreground(colorOrange).Render(
		fmt.Sprintf("Mode: [%s] (O/D/N)", modeName))
	cur := "Current:" + field(0, "Basics") + field(1, "Subskill")
	tgt := "Target: " + field(2, "Basics") + field(3, "Subskill") + "    " + mode
	return cur + "\n" + tgt
}

// renderRBTable produces a labelled table of rank-bonus values for
// the given (basics, subskill) under the screen's current mode.
func (s RBCalcScreen) renderRBTable(label string, basics, sub int) string {
	headerStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	rowLabelStyle := lipgloss.NewStyle().Foreground(colorDim)
	cellStyle := lipgloss.NewStyle()

	difficulties := []calc.Difficulty{
		calc.DifficultyBasic, calc.DifficultyEasy, calc.DifficultyAverage,
		calc.DifficultyDifficult, calc.DifficultyImpossible,
	}
	diffNames := []string{"Basic", "Easy", "Avg", "Diff", "Impos."}

	var b strings.Builder
	b.WriteString(headerStyle.Render(label))
	b.WriteByte('\n')

	// Column header.
	b.WriteString("        ")
	for _, n := range diffNames {
		b.WriteString(fmt.Sprintf("%8s", n))
	}
	b.WriteByte('\n')

	switch s.mode {
	case calc.ModeNoncombat:
		// Single row, no posture label.
		b.WriteString("        ")
		for _, d := range difficulties {
			v := calc.RankBonus(s.mode, basics, sub, 0, d)
			b.WriteString(fmt.Sprintf("%8s", formatRB(v)))
		}
		b.WriteByte('\n')
	default:
		var postures []calc.Posture
		var rowNames []string
		if s.mode == calc.ModeDefensive {
			postures = []calc.Posture{
				calc.PostureDefensive, calc.PostureWary, calc.PostureNormal,
				calc.PostureAggressive, calc.PostureBerserk,
			}
			rowNames = []string{"Def.", "Wary", "Norm.", "Aggr.", "Bers."}
		} else { // Offensive
			postures = []calc.Posture{
				calc.PostureBerserk, calc.PostureAggressive, calc.PostureNormal,
				calc.PostureWary, calc.PostureDefensive,
			}
			rowNames = []string{"Bers.", "Aggr.", "Norm.", "Wary", "Def."}
		}
		for i, p := range postures {
			b.WriteString(rowLabelStyle.Render(fmt.Sprintf("  %5s ", rowNames[i])))
			for _, d := range difficulties {
				v := calc.RankBonus(s.mode, basics, sub, p, d)
				b.WriteString(cellStyle.Render(fmt.Sprintf("%8s", formatRB(v))))
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatRB renders an RB value with up to 1 decimal place, dropping
// trailing zeros (matches the wiki calculator's display).
func formatRB(v float64) string {
	if v == float64(int(v)) {
		return strconv.Itoa(int(v))
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}
