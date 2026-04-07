package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// MetricsPane displays the current session metrics and a history table.
type MetricsPane struct {
	width   int
	height  int
	status  types.StatusUpdateEvent
	current *types.MetricSnapshot
	history []types.MetricSnapshot
}

// NewMetricsPane creates a new MetricsPane.
func NewMetricsPane() MetricsPane {
	return MetricsPane{}
}

// SetSize updates the pane dimensions.
func (m *MetricsPane) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// UpdateStatus updates the status data shown in the metrics pane.
func (m *MetricsPane) UpdateStatus(s types.StatusUpdateEvent) {
	m.status = s
}

// UpdateMetrics updates the current session and history data.
func (m *MetricsPane) UpdateMetrics(current *types.MetricSnapshot, history []types.MetricSnapshot) {
	m.current = current
	m.history = history
}

// View renders the metrics dashboard.
func (m MetricsPane) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	valueStyle := lipgloss.NewStyle().Foreground(colorGreen)

	var b strings.Builder

	// Current session header
	b.WriteString(headerStyle.Render("Current Session"))
	b.WriteByte('\n')

	if m.current != nil {
		dur := m.current.Duration()
		minutes := int(dur.Minutes())
		seconds := int(dur.Seconds()) % 60

		b.WriteString(fmt.Sprintf("%s %s\n",
			labelStyle.Render("Mode:"),
			valueStyle.Render(m.current.Mode)))
		b.WriteString(fmt.Sprintf("%s %s\n",
			labelStyle.Render("Duration:"),
			valueStyle.Render(fmt.Sprintf("%dm %ds", minutes, seconds))))

		// Display all tracked metrics.
		for _, entry := range m.current.Entries {
			b.WriteString(fmt.Sprintf("%s %s\n",
				labelStyle.Render(entry.Label+":"),
				valueStyle.Render(fmt.Sprintf("%d", entry.Value))))
		}
	} else {
		b.WriteString(labelStyle.Render("  No active session"))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	// History
	b.WriteString(headerStyle.Render("Session History"))
	b.WriteByte('\n')

	if len(m.history) == 0 {
		b.WriteString(labelStyle.Render("  No history"))
		b.WriteByte('\n')
	} else {
		// Show most recent sessions (limit to available space)
		maxRows := m.height - 12
		if maxRows < 1 {
			maxRows = 1
		}
		start := 0
		if len(m.history) > maxRows {
			start = len(m.history) - maxRows
		}

		for i := start; i < len(m.history); i++ {
			s := m.history[i]
			dur := s.Duration()
			minutes := int(dur.Minutes())
			seconds := int(dur.Seconds()) % 60

			line := fmt.Sprintf("  %s %s",
				labelStyle.Render(s.Mode),
				valueStyle.Render(fmt.Sprintf("%dm %ds", minutes, seconds)))

			// Show metric summaries inline.
			for _, entry := range s.Entries {
				line += fmt.Sprintf("  %s=%d", entry.Label, entry.Value)
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// padRight pads a string to the given width with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
