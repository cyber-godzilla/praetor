package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MenuScriptDirsMsg struct{}

type ScriptDirsCloseMsg struct {
	Dirs    []string
	Changed bool
}

type ScriptDirsScreen struct {
	dirs    []string
	cursor  int
	adding  bool
	addBuf  string
	confirm bool
	width   int
	height  int
}

func NewScriptDirsScreen(dirs []string) ScriptDirsScreen {
	cp := make([]string, len(dirs))
	copy(cp, dirs)
	return ScriptDirsScreen{dirs: cp}
}

func (s *ScriptDirsScreen) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s ScriptDirsScreen) Update(msg tea.KeyMsg) (ScriptDirsScreen, tea.Cmd) {
	if s.confirm {
		switch msg.Type {
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					s.confirm = false
					if s.cursor < len(s.dirs) {
						s.dirs = append(s.dirs[:s.cursor], s.dirs[s.cursor+1:]...)
						if s.cursor >= len(s.dirs) && s.cursor > 0 {
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
			path := strings.TrimSpace(s.addBuf)
			s.adding = false
			s.addBuf = ""
			if path != "" {
				s.dirs = append(s.dirs, path)
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
		dirs := make([]string, len(s.dirs))
		copy(dirs, s.dirs)
		return s, func() tea.Msg {
			return ScriptDirsCloseMsg{Dirs: dirs, Changed: true}
		}
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil
	case tea.KeyDown:
		if s.cursor < len(s.dirs)-1 {
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
				if len(s.dirs) > 0 {
					s.confirm = true
				}
				return s, nil
			}
		}
	}
	return s, nil
}

func (s ScriptDirsScreen) View() string {
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

	b.WriteString(titleStyle.Render("Script Directories"))
	b.WriteString("\n\n")

	if len(s.dirs) == 0 {
		b.WriteString(dimStyle.Render("  No directories configured"))
		b.WriteByte('\n')
	} else {
		maxVisible := s.height - 14
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := viewportWindow(len(s.dirs), maxVisible, s.cursor)
		end := start + maxVisible
		if end > len(s.dirs) {
			end = len(s.dirs)
		}
		for i := start; i < end; i++ {
			dir := s.dirs[i]
			line := fmt.Sprintf("%d. %s", i+1, dir)
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
			fmt.Sprintf("  Remove %q? [Y/N]", s.dirs[s.cursor])))
	} else if s.adding {
		editStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
		b.WriteString(editStyle.Render("  Path: "))
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render(s.addBuf + "\u2588"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("[Enter] add  [Esc] cancel"))
	} else {
		b.WriteString(dimStyle.Render("[\u2191/\u2193] navigate  [A] add  [D] remove  [Esc] save & close"))
	}

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
