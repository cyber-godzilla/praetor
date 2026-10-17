package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/wounds"
)

var woundFamilyNames = []string{"red", "purple", "blue", "orange"}

type MenuWoundSettingsMsg struct{}

type WoundSettingsCloseMsg struct {
	Config  config.WoundHighlightConfig
	Changed bool
}

type WoundSettingsScreen struct {
	cfg      config.WoundHighlightConfig
	original config.WoundHighlightConfig
	cursor   int
	width    int
	height   int
}

func NewWoundSettingsScreen(cfg config.WoundHighlightConfig) WoundSettingsScreen {
	return WoundSettingsScreen{cfg: cfg, original: cfg}
}

func (s *WoundSettingsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s WoundSettingsScreen) Update(msg tea.KeyMsg) (WoundSettingsScreen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		result := WoundSettingsCloseMsg{Config: s.cfg, Changed: s.cfg != s.original}
		return s, func() tea.Msg { return result }
	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
	case tea.KeyDown:
		if s.cursor < 4 {
			s.cursor++
		}
	case tea.KeyLeft:
		if s.cursor > 0 {
			s.cycleFamily(-1)
		}
	case tea.KeyRight:
		if s.cursor > 0 {
			s.cycleFamily(1)
		}
	case tea.KeyEnter, tea.KeySpace:
		if s.cursor == 0 {
			s.cfg.Enabled = !s.cfg.Enabled
		} else {
			s.cycleFamily(1)
		}
	}
	return s, nil
}

func (s *WoundSettingsScreen) cycleFamily(direction int) {
	value := s.familyAtCursor()
	index := 0
	for i, family := range woundFamilyNames {
		if family == value {
			index = i
			break
		}
	}
	index = (index + direction + len(woundFamilyNames)) % len(woundFamilyNames)
	s.setFamilyAtCursor(woundFamilyNames[index])
}

func (s WoundSettingsScreen) familyAtCursor() string {
	switch s.cursor {
	case 1:
		return s.cfg.Cuts
	case 2:
		return s.cfg.Bruises
	case 3:
		return s.cfg.Punctures
	case 4:
		return s.cfg.Burns
	default:
		return ""
	}
}

func (s *WoundSettingsScreen) setFamilyAtCursor(family string) {
	switch s.cursor {
	case 1:
		s.cfg.Cuts = family
	case 2:
		s.cfg.Bruises = family
	case 3:
		s.cfg.Punctures = family
	case 4:
		s.cfg.Burns = family
	}
}

func (s WoundSettingsScreen) View() string {
	title := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("Wound Highlighting")
	dim := lipgloss.NewStyle().Foreground(colorDim)
	selected := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxWidth := s.width - 10
	if boxWidth < 48 {
		boxWidth = 48
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(dim.Render("[↑/↓] select  [←/→/Enter] change  [Esc] save") + "\n\n")

	enabled := "OFF"
	if s.cfg.Enabled {
		enabled = "ON"
	}
	rows := []struct {
		label string
		value string
	}{
		{"Highlight wounds", enabled},
		{"Cuts", s.cfg.Cuts},
		{"Bruises / fractures", s.cfg.Bruises},
		{"Punctures / wounds", s.cfg.Punctures},
		{"Burns", s.cfg.Burns},
	}
	for i, row := range rows {
		prefix := "  "
		labelStyle := dim
		if i == s.cursor {
			prefix = "> "
			labelStyle = selected
		}
		value := row.value
		if i > 0 {
			value = renderWoundFamily(row.value)
		}
		b.WriteString(fmt.Sprintf("%s%-24s %s\n", prefix, labelStyle.Render(row.label), value))
	}
	b.WriteString("\n" + dim.Render("Verified singular phrases only. Severity sets the shade; levels 4–5 are bold."))
	return box.Render(b.String())
}

func renderWoundFamily(family string) string {
	var parts []string
	for severity := 1; severity <= 5; severity++ {
		color, ok := wounds.FamilyColor(family, severity)
		if !ok {
			continue
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		if severity >= 4 {
			style = style.Bold(true)
		}
		parts = append(parts, style.Render(fmt.Sprintf("%d", severity)))
	}
	return family + "  " + strings.Join(parts, " ")
}
