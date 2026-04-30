package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpSearchMsg is sent when the user submits a help search query.
// The wrapper will send ?<query> to the game server.
type HelpSearchMsg struct {
	Query string
}

// HelpCloseMsg is sent when the help screen is dismissed.
type HelpCloseMsg struct{}

// HelpScreen displays the help overlay with wiki link, search, and reference.
type HelpScreen struct {
	width     int
	height    int
	searching bool
	searchBuf string
	scroll    int
}

func NewHelpScreen() HelpScreen {
	return HelpScreen{}
}

func (h *HelpScreen) SetSize(w, hh int) {
	h.width = w
	h.height = hh
}

func (h HelpScreen) Update(msg tea.KeyMsg) (HelpScreen, tea.Cmd) {
	if h.searching {
		return h.updateSearch(msg)
	}

	switch msg.Type {
	case tea.KeyEscape:
		return h, func() tea.Msg { return HelpCloseMsg{} }

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'w':
				return h, func() tea.Msg {
					return HelpSearchMsg{Query: "__wiki__"}
				}
			case 's':
				h.searching = true
				h.searchBuf = ""
				return h, nil
			}
		}

	case tea.KeyUp:
		h.scroll--
		if h.scroll < 0 {
			h.scroll = 0
		}
		return h, nil

	case tea.KeyDown:
		h.scroll++
		return h, nil
	}

	return h, nil
}

func (h HelpScreen) updateSearch(msg tea.KeyMsg) (HelpScreen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		h.searching = false
		return h, nil

	case tea.KeyEnter:
		query := strings.TrimSpace(h.searchBuf)
		h.searching = false
		h.searchBuf = ""
		if query != "" {
			return h, func() tea.Msg {
				return HelpSearchMsg{Query: query}
			}
		}
		return h, nil

	case tea.KeyBackspace:
		if len(h.searchBuf) > 0 {
			h.searchBuf = h.searchBuf[:len(h.searchBuf)-1]
		}
		return h, nil

	case tea.KeyRunes:
		h.searchBuf += string(msg.Runes)
		return h, nil
	}

	return h, nil
}

func (h HelpScreen) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorDim)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	ruleStyle := lipgloss.NewStyle().Foreground(colorDim)

	boxWidth := h.width - 10
	if boxWidth < 30 {
		boxWidth = 30
	}
	innerWidth := boxWidth - 8 // account for border + padding
	rule := ruleStyle.Render(strings.Repeat("─", innerWidth))

	var lines []string

	lines = append(lines, titleStyle.Render("Praetor Help"))
	lines = append(lines, rule)

	// Slash commands
	lines = append(lines, headerStyle.Render("Slash Commands"))
	cmdEntries := []struct{ key, desc string }{
		{"/mode <name> [args]", "Set automation mode"},
		{"/list", "List all available automation modes"},
		{"/toggle <label>", "Toggle a boolean state value"},
		{"/set <label> <val>", "Set a state value"},
		{"/reconnect", "Disconnect and reconnect to game server"},
		{"/help", "Show this help screen"},
		{"/wiki [topic]", "Open wiki bookmarks (or a topic directly)"},
		{"/maps [location]", "Open map browser (or a location directly)"},
	}
	for _, e := range cmdEntries {
		lines = append(lines, "  "+keyStyle.Render(padRight(e.key, 22))+descStyle.Render(e.desc))
	}
	lines = append(lines, rule)

	// Keybindings
	lines = append(lines, headerStyle.Render("Keybindings"))
	keyEntries := []struct{ key, desc string }{
		{"Tab / Shift+Tab", "Next / previous tab"},
		{"Alt+1..9, Alt+0", "Jump to tab by number (0 = 10th)"},
		{"Alt+S", "Cycle display: sidebar → topbar → off"},
		{"Alt+M", "Quick-cycle automation mode"},
		{"Alt+I", "Toggle suppressed line reveal"},
		{"Alt+X", "Disable all automation"},
		{"Esc", "Open menu"},
		{"Ctrl+C", "Clear input / confirm quit"},
		{"PgUp / PgDn", "Scroll output"},
		{"Mouse wheel", "Scroll output (3 lines)"},
		{"Enter (empty)", "Send blank line to server"},
	}
	for _, e := range keyEntries {
		lines = append(lines, "  "+keyStyle.Render(padRight(e.key, 22))+descStyle.Render(e.desc))
	}
	lines = append(lines, rule)

	// Menu options
	lines = append(lines, headerStyle.Render("Menu (Esc)"))
	menuEntries := []struct{ key, desc string }{
		{"Reload Scripts", "Hot-reload all Lua modes"},
		{"Script Directories", "Configure where Lua modes load from"},
		{"Quick-Cycle Modes", "Configure Alt+M mode list"},
		{"Priority Commands", "Commands that jump the queue"},
		{"Highlights", "Manage string highlighting patterns"},
		{"Custom Tabs", "Configure custom tab filters"},
		{"Ignorelist (OOC)", "Suppress OOC by account name"},
		{"Ignorelist (Think)", "Suppress think aloud by character name"},
		{"Colorwords", "Toggle color word rendering"},
		{"Echo Typed Commands", "Toggle echo of user-typed commands"},
		{"Echo Script Commands", "Toggle echo of script-sent commands"},
		{"Hide IP Addresses", "Mask IP addresses in output"},
		{"Auto Reconnect", "Toggle auto-reconnect on disconnect"},
		{"Notification Settings", "Desktop notification thresholds + patterns"},
		{"Game Logs", "Toggle session log recording"},
		{"Persistent Data", "View/clear per-mode saved state"},
	}
	for _, e := range menuEntries {
		lines = append(lines, "  "+keyStyle.Render(padRight(e.key, 22))+descStyle.Render(e.desc))
	}

	// Footer (always visible, not part of scrollable content)
	var footer string
	if h.searching {
		footer = descStyle.Render("Search: "+h.searchBuf+"█") + "  " + dimStyle.Render("[Enter] search  [Esc] cancel")
	} else {
		footer = dimStyle.Render("[↑/↓] scroll  [W] wiki  [S] search help  [Esc] close")
	}

	// Apply scroll with clamping
	maxVisible := h.height - 6 // box border + padding + footer
	if maxVisible < 5 {
		maxVisible = 5
	}
	maxScroll := len(lines) - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scroll > maxScroll {
		h.scroll = maxScroll
	}
	if h.scroll < 0 {
		h.scroll = 0
	}
	start := h.scroll
	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[start:end]

	// Join content + footer
	content := strings.Join(visible, "\n") + "\n\n" + footer

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	return lipgloss.Place(h.width, h.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(content))
}
