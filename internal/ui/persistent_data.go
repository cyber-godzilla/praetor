package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PersistentKeyInfo struct {
	Key          string
	ValueSummary string
}

type MenuPersistentDataMsg struct{}
type PersistentDataSnapshotMsg struct {
	Username string
	Keys     []PersistentKeyInfo
}
type PersistentDataExportMsg struct{ Keys []string }
type PersistentDataClearMsg struct{ Keys []string }
type PersistentDataCloseMsg struct{}

type PersistentDataScreen struct {
	username string
	keys     []PersistentKeyInfo
	selected []bool
	cursor   int
	confirm  bool
	message  string
	width    int
	height   int
}

func NewPersistentDataScreen(username string, keys []PersistentKeyInfo) PersistentDataScreen {
	return PersistentDataScreen{
		username: username,
		keys:     keys,
		selected: make([]bool, len(keys)),
	}
}

func (p *PersistentDataScreen) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *PersistentDataScreen) SetMessage(msg string) {
	p.message = msg
}

func (p PersistentDataScreen) selectedKeys() []string {
	var keys []string
	for i, sel := range p.selected {
		if sel {
			keys = append(keys, p.keys[i].Key)
		}
	}
	return keys
}

func (p PersistentDataScreen) Update(msg tea.KeyMsg) (PersistentDataScreen, tea.Cmd) {
	if p.confirm {
		switch msg.Type {
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					p.confirm = false
					keys := p.selectedKeys()
					return p, func() tea.Msg { return PersistentDataClearMsg{Keys: keys} }
				case 'n', 'N':
					p.confirm = false
					return p, nil
				}
			}
		case tea.KeyEscape:
			p.confirm = false
			return p, nil
		}
		return p, nil
	}

	switch msg.Type {
	case tea.KeyEscape:
		return p, func() tea.Msg { return PersistentDataCloseMsg{} }
	case tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return p, nil
	case tea.KeyDown:
		if p.cursor < len(p.keys)-1 {
			p.cursor++
		}
		return p, nil
	case tea.KeySpace:
		if p.cursor < len(p.selected) {
			p.selected[p.cursor] = !p.selected[p.cursor]
		}
		return p, nil
	case tea.KeyEnter:
		keys := p.selectedKeys()
		if len(keys) > 0 {
			return p, func() tea.Msg { return PersistentDataExportMsg{Keys: keys} }
		}
		return p, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && (msg.Runes[0] == 'd' || msg.Runes[0] == 'D') {
			keys := p.selectedKeys()
			if len(keys) > 0 {
				p.confirm = true
			}
			return p, nil
		}
	}
	return p, nil
}

func (p PersistentDataScreen) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorDim)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(p.width - 10)

	var b strings.Builder

	title := "Persistent Data"
	if p.username != "" {
		title += " — " + p.username
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if len(p.keys) == 0 {
		b.WriteString(dimStyle.Render("  No persistent data"))
		b.WriteByte('\n')
	} else {
		maxVisible := p.height - 14
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := viewportWindow(len(p.keys), maxVisible, p.cursor)
		end := start + maxVisible
		if end > len(p.keys) {
			end = len(p.keys)
		}
		for i := start; i < end; i++ {
			key := p.keys[i]
			check := "[ ]"
			if p.selected[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("%s %s", check, key.Key)
			if key.ValueSummary != "" {
				line += "  " + dimStyle.Render(key.ValueSummary)
			}
			if i == p.cursor {
				b.WriteString(selectedStyle.Render("> " + line))
			} else {
				b.WriteString(normalStyle.Render("  " + line))
			}
			b.WriteByte('\n')
		}
	}

	if p.message != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("  " + p.message))
	}

	b.WriteString("\n\n")
	if p.confirm {
		count := len(p.selectedKeys())
		b.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render(
			fmt.Sprintf("  Clear %d item(s)? [Y/N]", count)))
	} else {
		b.WriteString(dimStyle.Render("  [Space] toggle  [Enter] Export selected  [D] Clear selected  [Esc] Back"))
	}

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
