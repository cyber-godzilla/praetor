package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Menu action messages.
type MenuReloadScriptsMsg struct{}
type MenuQuickCycleMsg struct{}           // open the quick-cycle mode picker
type MenuColorWordsMsg struct{}           // toggle color words
type MenuEchoTypedMsg struct{}            // toggle echo of user-typed commands
type MenuEchoScriptMsg struct{}           // toggle echo of script-sent commands
type MenuAutoReconnectMsg struct{}        // toggle auto reconnect
type MenuHideIPsMsg struct{}              // toggle IP address masking
type MenuGameLogsMsg struct{}             // toggle session logging
type MenuNotificationSettingsMsg struct{} // open notification settings
type MenuLogPathMsg struct {
	Path string
}
type MenuQuitMsg struct{}

// ScriptsReloadedMsg is sent by the wrapper after scripts have been reloaded.
type ScriptsReloadedMsg struct {
	Error error
}
type MenuCloseMsg struct{} // close menu without action

// menuItem is a single menu entry — either a selectable item or a section header.
type menuItem struct {
	label    string
	isHeader bool
	action   func() tea.Msg
}

// Menu is an overlay menu triggered by Esc.
type Menu struct {
	items       []menuItem
	cursor      int
	width       int
	height      int
	editingPath bool   // true when editing log path inline
	pathBuf     string // buffer for path editing
	message     string // transient status message, cleared on next keypress
}

func NewMenu(colorWords, echoTyped, echoScript, autoReconnect, hideIPs, gameLogs bool, logPath string, modesAvailable bool) Menu {
	cwLabel := "Colorwords: OFF"
	if colorWords {
		cwLabel = "Colorwords: ON"
	}
	echoTypedLabel := "Echo Typed Commands: OFF"
	if echoTyped {
		echoTypedLabel = "Echo Typed Commands: ON"
	}
	echoScriptLabel := "Echo Script Commands: OFF"
	if echoScript {
		echoScriptLabel = "Echo Script Commands: ON"
	}
	reconLabel := "Auto Reconnect: OFF"
	if autoReconnect {
		reconLabel = "Auto Reconnect: ON"
	}
	ipLabel := "Hide IP Addresses: OFF"
	if hideIPs {
		ipLabel = "Hide IP Addresses: ON"
	}
	logLabel := "Game Logs: OFF"
	if gameLogs {
		logLabel = "Game Logs: ON"
	}
	pathLabel := "Log Location: (default)"
	if logPath != "" {
		pathLabel = "Log Location: " + logPath
	}

	items := []menuItem{
		{label: "Scripts", isHeader: true},
		{label: "Reload Scripts", action: func() tea.Msg { return MenuReloadScriptsMsg{} }},
		{label: "Script Directories", action: func() tea.Msg { return MenuScriptDirsMsg{} }},
	}
	if modesAvailable {
		items = append(items, menuItem{label: "Quick-Cycle Modes", action: func() tea.Msg { return MenuQuickCycleMsg{} }})
	}
	items = append(items,
		menuItem{label: "Priority Commands", action: func() tea.Msg { return MenuPriorityCmdsMsg{} }},
		menuItem{label: "", isHeader: true},

		menuItem{label: "Display", isHeader: true},
		menuItem{label: "Highlights", action: func() tea.Msg { return MenuHighlightsMsg{} }},
		menuItem{label: "Custom Tabs", action: func() tea.Msg { return MenuTabsMsg{} }},
		menuItem{label: cwLabel, action: func() tea.Msg { return MenuColorWordsMsg{} }},
		menuItem{label: echoTypedLabel, action: func() tea.Msg { return MenuEchoTypedMsg{} }},
		menuItem{label: echoScriptLabel, action: func() tea.Msg { return MenuEchoScriptMsg{} }},
		menuItem{label: ipLabel, action: func() tea.Msg { return MenuHideIPsMsg{} }},
		menuItem{label: "", isHeader: true},

		menuItem{label: "Connection", isHeader: true},
		menuItem{label: reconLabel, action: func() tea.Msg { return MenuAutoReconnectMsg{} }},
		menuItem{label: "", isHeader: true},

		menuItem{label: "Notifications", isHeader: true},
		menuItem{label: "Notification Settings", action: func() tea.Msg { return MenuNotificationSettingsMsg{} }},
		menuItem{label: "", isHeader: true},

		menuItem{label: "Logs", isHeader: true},
		menuItem{label: logLabel, action: func() tea.Msg { return MenuGameLogsMsg{} }},
		menuItem{label: pathLabel, action: nil}, // handled specially — enters edit mode
		menuItem{label: "", isHeader: true},

		menuItem{label: "Data", isHeader: true},
		menuItem{label: "Persistent Data", action: func() tea.Msg { return MenuPersistentDataMsg{} }},
		menuItem{label: "", isHeader: true},

		menuItem{label: "Quit", action: func() tea.Msg { return MenuQuitMsg{} }},
	)

	m := Menu{items: items, pathBuf: logPath}
	// Set cursor to first selectable item.
	for i, item := range items {
		if !item.isHeader {
			m.cursor = i
			break
		}
	}
	return m
}

// isLogPathItem returns true if the cursor is on the log path item.
func (m Menu) isLogPathItem() bool {
	return m.items[m.cursor].label != "" &&
		strings.HasPrefix(m.items[m.cursor].label, "Log Location:")
}

func (m *Menu) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetMessage sets a transient status message on the menu.
func (m *Menu) SetMessage(msg string) {
	m.message = msg
}

func (m Menu) Update(msg tea.KeyMsg) (Menu, tea.Cmd) {
	// Clear transient message on any keypress.
	m.message = ""

	if m.editingPath {
		return m.updatePathEdit(msg)
	}

	switch msg.Type {
	case tea.KeyEscape:
		return m, func() tea.Msg { return MenuCloseMsg{} }

	case tea.KeyUp:
		for i := m.cursor - 1; i >= 0; i-- {
			if !m.items[i].isHeader {
				m.cursor = i
				break
			}
		}
		return m, nil

	case tea.KeyDown:
		for i := m.cursor + 1; i < len(m.items); i++ {
			if !m.items[i].isHeader {
				m.cursor = i
				break
			}
		}
		return m, nil

	case tea.KeyEnter:
		if m.isLogPathItem() {
			m.editingPath = true
			return m, nil
		}
		item := m.items[m.cursor]
		if item.action != nil {
			return m, item.action
		}
	}

	return m, nil
}

func (m Menu) updatePathEdit(msg tea.KeyMsg) (Menu, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.editingPath = false
		return m, nil

	case tea.KeyEnter:
		m.editingPath = false
		path := strings.TrimSpace(m.pathBuf)
		return m, func() tea.Msg { return MenuLogPathMsg{Path: path} }

	case tea.KeyBackspace:
		if len(m.pathBuf) > 0 {
			m.pathBuf = m.pathBuf[:len(m.pathBuf)-1]
		}
		return m, nil

	case tea.KeyRunes:
		m.pathBuf += string(msg.Runes)
		return m, nil

	case tea.KeySpace:
		m.pathBuf += " "
		return m, nil
	}
	return m, nil
}

func (m Menu) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true)

	headerStyle := lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(68)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Menu"))
	b.WriteString("\n\n")

	for i, item := range m.items {
		if item.isHeader {
			if item.label != "" {
				b.WriteString(headerStyle.Render("  " + item.label))
			}
			b.WriteByte('\n')
			continue
		}

		// Special rendering for log path in edit mode.
		if m.editingPath && m.isLogPathItem() && i == m.cursor {
			editStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
			dimStyle := lipgloss.NewStyle().Foreground(colorDim)
			b.WriteString(editStyle.Render("  > Log Location: "))
			b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render(m.pathBuf + "█"))
			b.WriteByte('\n')
			b.WriteString(dimStyle.Render("      [Enter] save  [Esc] cancel  (empty = default)"))
			b.WriteByte('\n')
			continue
		}

		if i == m.cursor {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorOrange).Bold(true).
				Render("  > " + item.label))
		} else {
			b.WriteString(lipgloss.NewStyle().
				Foreground(colorDim).
				Render("    " + item.label))
		}
		b.WriteByte('\n')
	}

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render("  " + m.message))
	}
	b.WriteString("\n")
	if !m.editingPath {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
			Render("[↑/↓] navigate  [Enter] select  [Esc] close"))
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
