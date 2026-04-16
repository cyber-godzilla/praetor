package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/config"
)

// NotificationSettingsCloseMsg is sent when the notification settings screen is dismissed.
type NotificationSettingsCloseMsg struct {
	Config  config.DesktopNotificationsConfig
	Changed bool
}

// notifyItemKind distinguishes item types in the list.
type notifyItemKind int

const (
	notifyItemThreshold notifyItemKind = iota
	notifyItemPattern
	notifyItemAdd
)

// notifyItem represents a single item in the notification settings list.
type notifyItem struct {
	kind  notifyItemKind
	index int // index into thresholds (0=health, 1=fatigue) or patterns slice
}

// notifyField tracks which field is being edited for patterns.
type notifyField int

const (
	fieldPattern notifyField = iota
	fieldTitle
	fieldMessage
)

// NotificationSettingsScreen manages notification threshold and pattern settings.
type NotificationSettingsScreen struct {
	healthBelow  config.ThresholdConfig
	fatigueBelow config.ThresholdConfig
	patterns     []config.NotifyPatternConfig

	items  []notifyItem
	cursor int

	editing   bool
	editBuf   string
	editField notifyField

	confirm bool
	changed bool

	width  int
	height int
}

func NewNotificationSettingsScreen(cfg config.DesktopNotificationsConfig) NotificationSettingsScreen {
	patterns := make([]config.NotifyPatternConfig, len(cfg.Patterns))
	copy(patterns, cfg.Patterns)

	s := NotificationSettingsScreen{
		healthBelow:  cfg.HealthBelow,
		fatigueBelow: cfg.FatigueBelow,
		patterns:     patterns,
	}
	s.rebuildItems()
	return s
}

func (s *NotificationSettingsScreen) rebuildItems() {
	s.items = nil
	s.items = append(s.items, notifyItem{kind: notifyItemThreshold, index: 0})
	s.items = append(s.items, notifyItem{kind: notifyItemThreshold, index: 1})
	for i := range s.patterns {
		s.items = append(s.items, notifyItem{kind: notifyItemPattern, index: i})
	}
	s.items = append(s.items, notifyItem{kind: notifyItemAdd})
}

func (s *NotificationSettingsScreen) SetSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *NotificationSettingsScreen) currentConfig() config.DesktopNotificationsConfig {
	patterns := make([]config.NotifyPatternConfig, len(s.patterns))
	copy(patterns, s.patterns)
	return config.DesktopNotificationsConfig{
		HealthBelow:  s.healthBelow,
		FatigueBelow: s.fatigueBelow,
		Patterns:     patterns,
	}
}

func (s *NotificationSettingsScreen) thresholdByIndex(idx int) *config.ThresholdConfig {
	if idx == 0 {
		return &s.healthBelow
	}
	return &s.fatigueBelow
}

func thresholdName(idx int) string {
	if idx == 0 {
		return "Health"
	}
	return "Fatigue"
}

func (s NotificationSettingsScreen) Update(msg tea.KeyMsg) (NotificationSettingsScreen, tea.Cmd) {
	if s.confirm {
		return s.updateConfirm(msg)
	}
	if s.editing {
		return s.updateEditing(msg)
	}

	switch msg.Type {
	case tea.KeyEscape:
		cfg := s.currentConfig()
		changed := s.changed
		return s, func() tea.Msg { return NotificationSettingsCloseMsg{Config: cfg, Changed: changed} }

	case tea.KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil

	case tea.KeyDown:
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
		return s, nil

	case tea.KeySpace:
		item := s.items[s.cursor]
		switch item.kind {
		case notifyItemThreshold:
			t := s.thresholdByIndex(item.index)
			t.Enabled = !t.Enabled
			s.changed = true
		case notifyItemPattern:
			s.patterns[item.index].Enabled = !s.patterns[item.index].Enabled
			s.changed = true
		}
		return s, nil

	case tea.KeyEnter:
		item := s.items[s.cursor]
		switch item.kind {
		case notifyItemThreshold:
			t := s.thresholdByIndex(item.index)
			s.editing = true
			s.editBuf = strconv.Itoa(t.Threshold)
		case notifyItemPattern:
			s.editing = true
			s.editField = fieldPattern
			s.editBuf = s.patterns[item.index].Pattern
		case notifyItemAdd:
			s.changed = true
			s.patterns = append(s.patterns, config.NotifyPatternConfig{Enabled: true})
			s.rebuildItems()
			s.cursor = len(s.items) - 2 // new pattern is before Add item
			s.editing = true
			s.editField = fieldPattern
			s.editBuf = ""
		}
		return s, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'd', 'D':
				item := s.items[s.cursor]
				if item.kind == notifyItemPattern {
					s.confirm = true
				}
			case ' ':
				// Toggle (fallback for terminals that send space as rune).
				item := s.items[s.cursor]
				switch item.kind {
				case notifyItemThreshold:
					t := s.thresholdByIndex(item.index)
					t.Enabled = !t.Enabled
					s.changed = true
				case notifyItemPattern:
					s.patterns[item.index].Enabled = !s.patterns[item.index].Enabled
					s.changed = true
				}
			}
		}
		return s, nil
	}

	return s, nil
}

func (s NotificationSettingsScreen) updateConfirm(msg tea.KeyMsg) (NotificationSettingsScreen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				s.confirm = false
				item := s.items[s.cursor]
				if item.kind == notifyItemPattern && item.index < len(s.patterns) {
					s.changed = true
					s.patterns = append(s.patterns[:item.index], s.patterns[item.index+1:]...)
					s.rebuildItems()
					if s.cursor >= len(s.items) {
						s.cursor = len(s.items) - 1
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

func (s NotificationSettingsScreen) updateEditing(msg tea.KeyMsg) (NotificationSettingsScreen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		item := s.items[s.cursor]
		if item.kind == notifyItemPattern && s.editField == fieldPattern && s.patterns[item.index].Pattern == "" {
			// New pattern with empty pattern field — remove it.
			s.patterns = append(s.patterns[:item.index], s.patterns[item.index+1:]...)
			s.rebuildItems()
			if s.cursor >= len(s.items) {
				s.cursor = len(s.items) - 1
			}
		}
		s.editing = false
		return s, nil

	case tea.KeyBackspace:
		if len(s.editBuf) > 0 {
			s.editBuf = s.editBuf[:len(s.editBuf)-1]
		}
		return s, nil

	case tea.KeyRunes:
		s.editBuf += string(msg.Runes)
		return s, nil

	case tea.KeyEnter:
		item := s.items[s.cursor]
		switch item.kind {
		case notifyItemThreshold:
			val, err := strconv.Atoi(strings.TrimSpace(s.editBuf))
			if err == nil {
				if val < 0 {
					val = 0
				}
				if val > 100 {
					val = 100
				}
				t := s.thresholdByIndex(item.index)
				t.Threshold = val
				s.changed = true
			}
			s.editing = false

		case notifyItemPattern:
			p := &s.patterns[item.index]
			value := strings.TrimSpace(s.editBuf)
			switch s.editField {
			case fieldPattern:
				p.Pattern = value
				s.changed = true
				if value == "" {
					// Empty pattern on confirm — remove entry.
					s.patterns = append(s.patterns[:item.index], s.patterns[item.index+1:]...)
					s.rebuildItems()
					if s.cursor >= len(s.items) {
						s.cursor = len(s.items) - 1
					}
					s.editing = false
				} else {
					s.editField = fieldTitle
					s.editBuf = p.Title
				}
			case fieldTitle:
				p.Title = value
				s.changed = true
				s.editField = fieldMessage
				s.editBuf = p.Message
			case fieldMessage:
				p.Message = value
				s.changed = true
				s.editing = false
			}
		}
		return s, nil
	}

	return s, nil
}

func (s NotificationSettingsScreen) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cccccc"))
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	enabledStyle := lipgloss.NewStyle().Foreground(colorOrange)
	disabledStyle := lipgloss.NewStyle().Foreground(colorDim)

	boxWidth := s.width - 10
	if boxWidth < 40 {
		boxWidth = 40
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 2).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Notification Settings"))
	b.WriteString("\n\n")

	// Viewport calculation.
	totalItems := len(s.items)
	maxVisible := s.height - 12
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := viewportWindow(totalItems, maxVisible, s.cursor)
	end := start + maxVisible
	if end > totalItems {
		end = totalItems
	}

	patternHeaderPrinted := false

	for idx := start; idx < end; idx++ {
		item := s.items[idx]

		// Print section headers as needed.
		if idx == start && item.kind == notifyItemThreshold {
			b.WriteString(headerStyle.Render("  Thresholds"))
			b.WriteByte('\n')
		}
		if !patternHeaderPrinted && (item.kind == notifyItemPattern || item.kind == notifyItemAdd) {
			patternHeaderPrinted = true
			b.WriteByte('\n')
			b.WriteString(headerStyle.Render("  Patterns"))
			b.WriteByte('\n')
		}

		cursor := "  "
		cursorStyle := normalStyle
		if idx == s.cursor {
			cursor = "> "
			cursorStyle = selectedStyle
		}

		switch item.kind {
		case notifyItemThreshold:
			t := s.thresholdByIndex(item.index)
			indicator := disabledStyle.Render("○")
			if t.Enabled {
				indicator = enabledStyle.Render("●")
			}

			name := thresholdName(item.index)
			if s.editing && idx == s.cursor {
				b.WriteString(cursorStyle.Render(cursor))
				b.WriteString(indicator)
				b.WriteString(" ")
				b.WriteString(cursorStyle.Render(fmt.Sprintf("%s below ", name)))
				b.WriteString(normalStyle.Render(s.editBuf + "█"))
				b.WriteString(cursorStyle.Render("%"))
			} else {
				b.WriteString(cursorStyle.Render(cursor))
				b.WriteString(indicator)
				b.WriteString(" ")
				b.WriteString(cursorStyle.Render(fmt.Sprintf("%s below %d%%", name, t.Threshold)))
			}
			b.WriteByte('\n')

		case notifyItemPattern:
			p := s.patterns[item.index]
			indicator := disabledStyle.Render("○")
			if p.Enabled {
				indicator = enabledStyle.Render("●")
			}

			if s.confirm && idx == s.cursor {
				b.WriteString(lipgloss.NewStyle().Foreground(colorRed).Render(
					fmt.Sprintf("  Delete %q? [Y/N]", p.Pattern)))
				b.WriteByte('\n')
			} else if s.editing && idx == s.cursor {
				var fieldName string
				switch s.editField {
				case fieldPattern:
					fieldName = "Pattern"
				case fieldTitle:
					fieldName = "Title"
				case fieldMessage:
					fieldName = "Message"
				}
				b.WriteString(cursorStyle.Render(cursor))
				b.WriteString(indicator)
				b.WriteString(" ")
				b.WriteString(cursorStyle.Render(fieldName + ": "))
				b.WriteString(normalStyle.Render(s.editBuf + "█"))
				b.WriteByte('\n')
			} else {
				title := p.Title
				message := p.Message
				if title == "" {
					title = `""`
				} else {
					title = fmt.Sprintf("%q", title)
				}
				if message == "" {
					message = `""`
				} else {
					message = fmt.Sprintf("%q", message)
				}
				b.WriteString(cursorStyle.Render(cursor))
				b.WriteString(indicator)
				b.WriteString(" ")
				b.WriteString(cursorStyle.Render(fmt.Sprintf("%s — %s / %s", p.Pattern, title, message)))
				b.WriteByte('\n')
			}

		case notifyItemAdd:
			if idx == s.cursor {
				b.WriteString(selectedStyle.Render(cursor + "+ Add new pattern..."))
			} else {
				b.WriteString(dimStyle.Render(cursor + "+ Add new pattern..."))
			}
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	if s.editing {
		b.WriteString(dimStyle.Render("[Enter] confirm  [Esc] cancel"))
	} else {
		b.WriteString(dimStyle.Render("[Space] toggle  [Enter] edit  [D] delete  [Esc] save & back"))
	}

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
