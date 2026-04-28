package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SetModeMsg signals the wrapper/client to switch the automation mode.
type SetModeMsg struct {
	Mode string
	Args []string
}

// QuickCycle manages a list of modes that Alt+M cycles through.
type QuickCycle struct {
	modes []string
	index int // current position in the cycle
}

// NewQuickCycle creates a QuickCycle with the given default modes.
func NewQuickCycle(modes []string) QuickCycle {
	return QuickCycle{
		modes: modes,
		index: 0,
	}
}

// Next returns the next mode in the cycle and advances the index.
func (qc *QuickCycle) Next() string {
	if len(qc.modes) == 0 {
		return "disable"
	}
	qc.index = (qc.index + 1) % len(qc.modes)
	return qc.modes[qc.index]
}

// Current returns the current mode in the cycle without advancing.
func (qc *QuickCycle) Current() string {
	if len(qc.modes) == 0 {
		return "disable"
	}
	return qc.modes[qc.index]
}

// Modes returns the current cycle list.
func (qc *QuickCycle) Modes() []string {
	return qc.modes
}

// SetModes replaces the cycle list and resets the index.
func (qc *QuickCycle) SetModes(modes []string) {
	qc.modes = modes
	qc.index = 0
}

// ModePicker lets the user select which modes are in the quick-cycle list.
// Shows all available modes with checkboxes.
type ModePicker struct {
	allModes []string // all available modes
	selected map[string]bool
	cursor   int
	width    int
	height   int
}

// ModePickerCloseMsg is sent when the mode picker is dismissed.
type ModePickerCloseMsg struct {
	Modes []string // the selected modes in order
}

func NewModePicker(allModes []string, currentCycle []string) ModePicker {
	sel := make(map[string]bool)
	for _, m := range currentCycle {
		sel[m] = true
	}
	return ModePicker{
		allModes: allModes,
		selected: sel,
	}
}

func (mp *ModePicker) SetSize(w, h int) {
	mp.width = w
	mp.height = h
}

func (mp ModePicker) Update(msg tea.KeyMsg) (ModePicker, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		// Save and close.
		var modes []string
		for _, m := range mp.allModes {
			if mp.selected[m] {
				modes = append(modes, m)
			}
		}
		return mp, func() tea.Msg { return ModePickerCloseMsg{Modes: modes} }

	case tea.KeyUp:
		if mp.cursor > 0 {
			mp.cursor--
		}
		return mp, nil

	case tea.KeyDown:
		if mp.cursor < len(mp.allModes)-1 {
			mp.cursor++
		}
		return mp, nil

	case tea.KeyEnter, tea.KeyRunes:
		// Toggle space or enter.
		if msg.Type == tea.KeyEnter || (len(msg.Runes) == 1 && msg.Runes[0] == ' ') {
			if mp.cursor >= 0 && mp.cursor < len(mp.allModes) {
				mode := mp.allModes[mp.cursor]
				mp.selected[mode] = !mp.selected[mode]
			}
		}
		return mp, nil
	}

	return mp, nil
}

func (mp ModePicker) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxWidth := mp.width - 10
	if boxWidth < 36 {
		boxWidth = 36
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Quick-Cycle Modes"))
	b.WriteString("\n\n")

	maxVisible := mp.height - 12
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := viewportWindow(len(mp.allModes), maxVisible, mp.cursor)
	end := start + maxVisible
	if end > len(mp.allModes) {
		end = len(mp.allModes)
	}

	arrowStyle := lipgloss.NewStyle().Foreground(colorDim)
	hasAbove := start > 0
	hasBelow := end < len(mp.allModes)

	if hasAbove {
		b.WriteString(arrowStyle.Render("      ▲"))
		b.WriteByte('\n')
	}

	for i := start; i < end; i++ {
		mode := mp.allModes[i]
		check := "  "
		if mp.selected[mode] {
			check = lipgloss.NewStyle().Foreground(colorGreen).Render("✓ ")
		}
		label := mode
		style := lipgloss.NewStyle().Foreground(colorDim)
		if i == mp.cursor {
			style = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
			label = "  > " + label
		} else {
			label = "    " + label
		}
		b.WriteString(check)
		b.WriteString(style.Render(label))
		b.WriteByte('\n')
	}

	if hasBelow {
		b.WriteString(arrowStyle.Render("      ▼"))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render("[Space] toggle  [Enter] select  [Esc] save"))

	return lipgloss.Place(mp.width, mp.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
