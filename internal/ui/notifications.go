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
	Config config.DesktopNotificationsConfig
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

// Ensure imports are used.
var (
	_ = fmt.Sprintf
	_ = strconv.Itoa
	_ = strings.TrimSpace
	_ tea.Cmd
	_ lipgloss.Style
)
