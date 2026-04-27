package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MenuPriorityCmdsMsg struct{}

type PriorityCmdsCloseMsg struct {
	Cmds    []string
	Changed bool
}

type PriorityCmdsScreen struct {
	cmds    []string
	cursor  int
	adding  bool
	addBuf  string
	confirm bool
	width   int
	height  int
}

func NewPriorityCmdsScreen(cmds []string) PriorityCmdsScreen {
	cp := make([]string, len(cmds))
	copy(cp, cmds)
	return PriorityCmdsScreen{cmds: cp}
}

func (s *PriorityCmdsScreen) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s PriorityCmdsScreen) Update(msg tea.KeyMsg) (PriorityCmdsScreen, tea.Cmd) {
	if s.confirm {
		switch msg.Type {
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					s.confirm = false
					if s.cursor < len(s.cmds) {
						s.cmds = append(s.cmds[:s.cursor], s.cmds[s.cursor+1:]...)
						if s.cursor >= len(s.cmds) && s.cursor > 0 {
							s.cursor--
						}
					}
					return s, nil
				case 'n', 'N':
					s.confirm = false
					return s, nil
				}
			}
		case tea.KeyEscape:
			s.confirm = false
			return s, nil
		}
		return s, nil
	}

	if s.adding {
		switch msg.Type {
		case tea.KeyEscape:
			s.adding = false
			s.addBuf = ""
			return s, nil
		case tea.KeyEnter:
			cmd := strings.TrimSpace(s.addBuf)
			s.adding = false
			s.addBuf = ""
			if cmd != "" {
				s.cmds = append(s.cmds, cmd)
			}
			return s, nil
		case tea.KeyBackspace:
			if len(s.addBuf) > 0 {
				s.addBuf = s.addBuf[:len(s.addBuf)-1]
			}
			return s, nil
		case tea.KeyRunes:
			s.addBuf += string(msg.Runes)
			return s, nil
		case tea.KeySpace:
			s.addBuf += " "
			return s, nil
		}
		return s, nil
	}

	switch msg.Type {
	case tea.KeyEscape:
		cmds := make([]string, len(s.cmds))
		copy(cmds, s.cmds)
		return s, func() tea.Msg {
			return PriorityCmdsCloseMsg{Cmds: cmds, Changed: true}
		}
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil
	case tea.KeyDown:
		if s.cursor < len(s.cmds)-1 {
			s.cursor++
		}
		return s, nil
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'a', 'A':
				s.adding = true
				s.addBuf = ""
				return s, nil
			case 'd', 'D':
				if len(s.cmds) > 0 {
					s.confirm = true
				}
				return s, nil
			}
		}
	}
	return s, nil
}

func (s PriorityCmdsScreen) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorDim)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(s.width - 10)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Priority Commands"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Commands that jump to the front of the queue"))
	b.WriteString("\n\n")

	if len(s.cmds) == 0 {
		b.WriteString(dimStyle.Render("  No priority commands configured"))
		b.WriteByte('\n')
	} else {
		maxVisible := s.height - 14
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := viewportWindow(len(s.cmds), maxVisible, s.cursor)
		end := start + maxVisible
		if end > len(s.cmds) {
			end = len(s.cmds)
		}
		for i := start; i < end; i++ {
			cmd := s.cmds[i]
			line := fmt.Sprintf("%d. %s", i+1, cmd)
			if i == s.cursor {
				b.WriteString(selectedStyle.Render("  > " + line))
			} else {
				b.WriteString(normalStyle.Render("    " + line))
			}
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n")
	if s.confirm {
		b.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render(
			fmt.Sprintf("  Remove %q? [Y/N]", s.cmds[s.cursor])))
	} else if s.adding {
		editStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
		b.WriteString(editStyle.Render("  Command: "))
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render(s.addBuf + "█"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("[Enter] add  [Esc] cancel"))
	} else {
		b.WriteString(dimStyle.Render("[↑/↓] navigate  [A] add  [D] remove  [Esc] save & close"))
	}

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
