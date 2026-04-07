package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar displays connection status, mode, and vitals in a single line.
type StatusBar struct {
	mode         string
	health       int
	fatigue      int
	encumbrance  int
	connected    bool
	reconnecting bool
	attempt      int
	nextDelay    time.Duration
	width        int
}

// NewStatusBar creates a new StatusBar.
func NewStatusBar() StatusBar {
	return StatusBar{
		mode:    "disable",
		health:  100,
		fatigue: 100,
	}
}

// SetWidth updates the status bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

// UpdateMode updates the displayed mode name.
func (s *StatusBar) UpdateMode(mode string) {
	s.mode = mode
}

// UpdateVitals updates the health, fatigue, and encumbrance percentages.
func (s *StatusBar) UpdateVitals(health, fatigue, encumbrance *int) {
	if health != nil {
		s.health = *health
	}
	if fatigue != nil {
		s.fatigue = *fatigue
	}
	if encumbrance != nil {
		s.encumbrance = *encumbrance
	}
}

// SetConnected sets the connection status.
func (s *StatusBar) SetConnected(connected bool) {
	s.connected = connected
	if connected {
		s.reconnecting = false
	}
}

// SetReconnecting marks the status bar as reconnecting.
func (s *StatusBar) SetReconnecting(attempt int, nextDelay time.Duration) {
	s.reconnecting = true
	s.connected = false
	s.attempt = attempt
	s.nextDelay = nextDelay
}

// View renders the status bar.
func (s StatusBar) View() string {
	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	modeStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)

	// Left side: HP, FT, EN, connection status
	var leftParts []string

	leftParts = append(leftParts, labelStyle.Render("HP: ")+vitalColor(s.health).Render(fmt.Sprintf("%d%%", s.health)))
	leftParts = append(leftParts, labelStyle.Render("FT: ")+vitalColor(s.fatigue).Render(fmt.Sprintf("%d%%", s.fatigue)))
	leftParts = append(leftParts, labelStyle.Render("EN: ")+vitalColor(s.encumbrance).Render(fmt.Sprintf("%d%%", s.encumbrance)))

	var connStr string
	switch {
	case s.connected:
		connStr = lipgloss.NewStyle().Foreground(colorGreen).Render("Connected ●")
	case s.reconnecting:
		connStr = lipgloss.NewStyle().Foreground(colorOrange).Render(
			fmt.Sprintf("Reconnecting (%d, %s) ◌", s.attempt, s.nextDelay.Round(time.Second)))
	default:
		connStr = lipgloss.NewStyle().Foreground(colorRed).Render("Disconnected ○")
	}
	leftParts = append(leftParts, connStr)

	left := strings.Join(leftParts, "  ")

	// Right side: mode
	modeStr := s.mode
	if modeStr == "" {
		modeStr = "none"
	}
	right := labelStyle.Render("Mode: ") + modeStyle.Render(modeStr)

	// Space-between layout: left content + padding + right content
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := s.width - leftWidth - rightWidth - 2 // 2 for border padding
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + right
	return statusBarStyle.Width(s.width).Render(content)
}

// vitalColor returns a style with color based on the vital percentage.
func vitalColor(pct int) lipgloss.Style {
	switch {
	case pct > 50:
		return lipgloss.NewStyle().Foreground(colorGreen)
	case pct > 25:
		return lipgloss.NewStyle().Foreground(colorOrange)
	default:
		return lipgloss.NewStyle().Foreground(colorRed)
	}
}
