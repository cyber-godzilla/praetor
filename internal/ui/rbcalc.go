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
	healing       bool
	slotPage      int // 0 = slots 1-10, 1 = slots 11-20
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
		case 'L', 'l':
			s.selfTaught = !s.selfTaught
			return s, nil
		case 'H', 'h':
			s.healing = !s.healing
			return s, nil
		case 'S', 's':
			s.slotPage = 1 - s.slotPage
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

	curBasics, curSub, tgtBasics, tgtSub := s.parsedInputs()
	// Target RB table needs both inputs (RB combines basics + sub);
	// the cost panel only depends on the subskill range.
	hasTargetTable := tgtBasics > 0 && tgtSub > 0
	hasCostPanel := tgtSub > 0

	var left strings.Builder
	left.WriteString(s.renderRBTable("Current", curBasics, curSub))
	if hasTargetTable {
		left.WriteString("\n")
		left.WriteString(s.renderRBTable("Target", tgtBasics, tgtSub))
	}

	var body string
	if hasCostPanel {
		right := s.renderTrainingPanel(curBasics, curSub, tgtBasics, tgtSub)
		body = lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Padding(0, 2, 0, 0).Render(left.String()),
			right,
		)
	} else {
		body = left.String()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Rank Bonus Calculator"))
	b.WriteString("\n\n")
	b.WriteString(s.renderInputHeader())
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[Tab] Field   [O/D/N] Mode   [T] Self-Trained   [L] Self-Taught   [H] Healing   [S] Slots   [Esc] Close"))

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

// renderTrainingPanel produces the right-hand training-cost block.
func (s RBCalcScreen) renderTrainingPanel(curBasics, curSub, tgtBasics, tgtSub int) string {
	headerStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	onStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	offStyle := lipgloss.NewStyle().Foreground(colorDim)
	warnStyle := lipgloss.NewStyle().Foreground(colorGreen)

	deltaBasics := tgtBasics - curBasics
	deltaSub := tgtSub - curSub

	var b strings.Builder
	b.WriteString(headerStyle.Render("Training cost"))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("ΔBasics: %+d\n", deltaBasics))
	b.WriteString(fmt.Sprintf("ΔSubskill: %+d\n\n", deltaSub))

	toggleLine := func(key, label, desc string, on bool) string {
		state := "OFF"
		stateStyle := offStyle
		if on {
			state = "ON"
			stateStyle = onStyle
		}
		head := fmt.Sprintf("[%s] %s: %s", key, label, stateStyle.Render(state))
		return head + "\n    " + dimStyle.Render(desc) + "\n"
	}
	b.WriteString(toggleLine("T", "Self-Trained", "Using selftrain command", s.selfTrained))
	b.WriteString(toggleLine("L", "Self-Taught", "Has self-taught trait", s.selfTaught))
	b.WriteString(toggleLine("H", "Healing", "Healing", s.healing))
	b.WriteByte('\n')

	pageLabel := "Slots 1-10"
	startSlot, endSlot := 1, 10
	if s.slotPage == 1 {
		pageLabel = "Slots 11-20"
		startSlot, endSlot = 11, 20
	}
	b.WriteString(headerStyle.Render("Skill Point Cost to Train  ") +
		dimStyle.Render("("+pageLabel+", [S] toggles)"))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("%-6s%8s%8s%8s%8s%8s\n", "Slot", "Basic", "Easy", "Avg", "Diff", "Impos."))

	// Basic column tracks the basics rank delta; the rest track subskill.
	// DifficultyBasic computes as Easy in the calc package (same SP rate).
	difficulties := []calc.Difficulty{
		calc.DifficultyEasy, calc.DifficultyAverage,
		calc.DifficultyDifficult, calc.DifficultyImpossible,
	}
	for slot := startSlot; slot <= endSlot; slot++ {
		b.WriteString(fmt.Sprintf("%-6s", ordinal(slot)))
		basicCost := calc.TrainSPCost(curBasics, tgtBasics, slot, calc.DifficultyBasic,
			s.selfTrained, s.selfTaught, s.healing)
		b.WriteString(fmt.Sprintf(" %6d ", basicCost))
		for _, d := range difficulties {
			cost := calc.TrainSPCost(curSub, tgtSub, slot, d, s.selfTrained, s.selfTaught, s.healing)
			b.WriteString(fmt.Sprintf(" %6d ", cost))
		}
		b.WriteByte('\n')
	}

	// Ranks above 1150 always cost as if self-trained, regardless of the
	// toggle. The wiki cost formula already encodes that, so the numbers
	// above are correct either way — but warn the user so they know the
	// /selftrain command is required to actually train those ranks. The
	// rule applies equally to basics and subskills.
	if (tgtSub > 1150 || tgtBasics > 1150) && !s.selfTrained {
		b.WriteByte('\n')
		b.WriteString(warnStyle.Render("Note: ranks 1151+ require /selftrain (cost shown reflects this)"))
		b.WriteByte('\n')
	}
	return b.String()
}

// ordinal renders 1->"1st", 2->"2nd", 3->"3rd", 4->"4th", ..., 20->"20th".
func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}
