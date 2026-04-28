package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IgnorelistKind selects which list this screen edits. The kind
// determines the title, the empty-state copy, and which close-message
// type is emitted on Esc.
type IgnorelistKind int

const (
	IgnorelistKindOOC IgnorelistKind = iota
	IgnorelistKindThink
)

// MenuIgnorelistOOCMsg opens the OOC (account) ignorelist editor.
type MenuIgnorelistOOCMsg struct{}

// MenuIgnorelistThinkMsg opens the Think (character) ignorelist editor.
type MenuIgnorelistThinkMsg struct{}

// IgnorelistOOCCloseMsg carries the edited OOC list back to the wrapper.
type IgnorelistOOCCloseMsg struct {
	Names   []string
	Changed bool
}

// IgnorelistThinkCloseMsg carries the edited Think list back to the wrapper.
type IgnorelistThinkCloseMsg struct {
	Names   []string
	Changed bool
}

// IgnorelistScreen is a generic add/remove editor backing both ignorelists.
type IgnorelistScreen struct {
	kind    IgnorelistKind
	names   []string
	cursor  int
	adding  bool
	addBuf  string
	confirm bool
	width   int
	height  int
}

// NewIgnorelistScreen creates an editor seeded with the given list.
// Names is copied; mutations on the screen never write back to the
// caller's slice until the close message is emitted.
func NewIgnorelistScreen(kind IgnorelistKind, names []string) IgnorelistScreen {
	cp := make([]string, len(names))
	copy(cp, names)
	return IgnorelistScreen{kind: kind, names: cp}
}

func (s *IgnorelistScreen) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s IgnorelistScreen) title() string {
	if s.kind == IgnorelistKindOOC {
		return "Ignorelist (OOC)"
	}
	return "Ignorelist (Think)"
}

func (s IgnorelistScreen) subtitle() string {
	if s.kind == IgnorelistKindOOC {
		return "  Suppress OOC channel by account name"
	}
	return "  Suppress think aloud by character name"
}

func (s IgnorelistScreen) emptyState() string {
	if s.kind == IgnorelistKindOOC {
		return "  No accounts ignored"
	}
	return "  No characters ignored"
}

func (s IgnorelistScreen) inputLabel() string {
	if s.kind == IgnorelistKindOOC {
		return "  Account: "
	}
	return "  Character: "
}

// alreadyContains reports whether the given name (case-insensitive) is
// already present in s.names.
func (s IgnorelistScreen) alreadyContains(name string) bool {
	lower := strings.ToLower(name)
	for _, existing := range s.names {
		if strings.ToLower(existing) == lower {
			return true
		}
	}
	return false
}

func (s IgnorelistScreen) Update(msg tea.KeyMsg) (IgnorelistScreen, tea.Cmd) {
	if s.confirm {
		switch msg.Type {
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'y', 'Y':
					s.confirm = false
					if s.cursor < len(s.names) {
						s.names = append(s.names[:s.cursor], s.names[s.cursor+1:]...)
						if s.cursor >= len(s.names) && s.cursor > 0 {
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
			name := strings.TrimSpace(s.addBuf)
			s.adding = false
			s.addBuf = ""
			if name != "" && !s.alreadyContains(name) {
				s.names = append(s.names, name)
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
		}
		return s, nil
	}

	switch msg.Type {
	case tea.KeyEscape:
		// Snapshot the slice locally so the cmd doesn't share backing
		// storage with the live screen. Mirrors priority_cmds.go.
		names := make([]string, len(s.names))
		copy(names, s.names)
		kind := s.kind
		return s, func() tea.Msg {
			if kind == IgnorelistKindOOC {
				return IgnorelistOOCCloseMsg{Names: names, Changed: true}
			}
			return IgnorelistThinkCloseMsg{Names: names, Changed: true}
		}
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil
	case tea.KeyDown:
		if s.cursor < len(s.names)-1 {
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
				if len(s.names) > 0 {
					s.confirm = true
				}
				return s, nil
			}
		}
	}
	return s, nil
}

func (s IgnorelistScreen) View() string {
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

	b.WriteString(titleStyle.Render(s.title()))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(s.subtitle()))
	b.WriteString("\n\n")

	if len(s.names) == 0 {
		b.WriteString(dimStyle.Render(s.emptyState()))
		b.WriteByte('\n')
	} else {
		maxVisible := s.height - 14
		if maxVisible < 3 {
			maxVisible = 3
		}
		start := viewportWindow(len(s.names), maxVisible, s.cursor)
		end := start + maxVisible
		if end > len(s.names) {
			end = len(s.names)
		}
		for i := start; i < end; i++ {
			line := fmt.Sprintf("%d. %s", i+1, s.names[i])
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
			fmt.Sprintf("  Remove %q? [Y/N]", s.names[s.cursor])))
	} else if s.adding {
		editStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
		b.WriteString(editStyle.Render(s.inputLabel()))
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render(s.addBuf + "█"))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("[Enter] add  [Esc] cancel"))
	} else {
		b.WriteString(dimStyle.Render("[↑/↓] navigate  [A] add  [D] remove  [Esc] save & close"))
	}

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
