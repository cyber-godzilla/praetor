package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/config"
)

// MenuTabsMsg opens the tab manager.
type MenuTabsMsg struct{}

// TabEditorCloseMsg is sent when the tab editor is dismissed.
type TabEditorCloseMsg struct {
	Tabs []config.CustomTabConfig
}

// tabEditorMode tracks which screen we're on.
type tabEditorMode int

const (
	temList    tabEditorMode = iota // list of custom tabs
	temEdit                         // editing a single tab
	temAddRule                      // entering a new rule pattern
	temNewTab                       // entering a new tab name
)

// TabEditor manages custom tabs — create, edit, delete, toggle visibility.
type TabEditor struct {
	tabs     []config.CustomTabConfig
	cursor   int
	mode     tabEditorMode
	editIdx  int    // index of tab being edited
	editCur  int    // cursor within the edit screen
	inputBuf string // text input buffer
	width    int
	height   int
}

func NewTabEditor(tabs []config.CustomTabConfig) TabEditor {
	cp := make([]config.CustomTabConfig, len(tabs))
	copy(cp, tabs)
	return TabEditor{tabs: cp}
}

func (te *TabEditor) SetSize(w, h int) {
	te.width = w
	te.height = h
}

func (te TabEditor) Update(msg tea.KeyMsg) (TabEditor, tea.Cmd) {
	switch te.mode {
	case temList:
		return te.updateList(msg)
	case temEdit:
		return te.updateEdit(msg)
	case temAddRule:
		return te.updateAddRule(msg)
	case temNewTab:
		return te.updateNewTab(msg)
	}
	return te, nil
}

func (te TabEditor) updateList(msg tea.KeyMsg) (TabEditor, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return te, func() tea.Msg {
			return TabEditorCloseMsg{Tabs: te.tabs}
		}
	case tea.KeyUp:
		if te.cursor > 0 {
			te.cursor--
		}
	case tea.KeyDown:
		max := len(te.tabs) // +1 for "Add new" would be at len(te.tabs)
		if te.cursor < max {
			te.cursor++
		}
	case tea.KeyEnter:
		if te.cursor == len(te.tabs) {
			// "Add new tab"
			te.mode = temNewTab
			te.inputBuf = ""
		} else {
			// Edit existing tab
			te.editIdx = te.cursor
			te.editCur = 0
			te.mode = temEdit
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'd':
				if te.cursor < len(te.tabs) {
					te.tabs = append(te.tabs[:te.cursor], te.tabs[te.cursor+1:]...)
					if te.cursor >= len(te.tabs) && te.cursor > 0 {
						te.cursor--
					}
				}
			case 'v':
				if te.cursor < len(te.tabs) {
					te.tabs[te.cursor].Visible = !te.tabs[te.cursor].Visible
				}
			}
		}
	case tea.KeySpace:
		if te.cursor < len(te.tabs) {
			te.tabs[te.cursor].Visible = !te.tabs[te.cursor].Visible
		}
	}
	return te, nil
}

func (te TabEditor) updateEdit(msg tea.KeyMsg) (TabEditor, tea.Cmd) {
	tab := &te.tabs[te.editIdx]
	switch msg.Type {
	case tea.KeyEscape:
		te.mode = temList
	case tea.KeyUp:
		if te.editCur > 0 {
			te.editCur--
		}
	case tea.KeyDown:
		max := len(tab.Rules) // "Add match" at len
		if te.editCur < max {
			te.editCur++
		}
	case tea.KeyEnter:
		if te.editCur == len(tab.Rules) {
			// "Add match"
			te.mode = temAddRule
			te.inputBuf = ""
		}
	case tea.KeySpace:
		if te.editCur < len(tab.Rules) {
			tab.Rules[te.editCur].Active = !tab.Rules[te.editCur].Active
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 1 && te.editCur < len(tab.Rules) {
			switch msg.Runes[0] {
			case 't':
				// Toggle include/exclude.
				tab.Rules[te.editCur].Include = !tab.Rules[te.editCur].Include
			case 'd':
				// Delete rule.
				tab.Rules = append(tab.Rules[:te.editCur], tab.Rules[te.editCur+1:]...)
				if te.editCur >= len(tab.Rules) && te.editCur > 0 {
					te.editCur--
				}
			}
		}
	}
	return te, nil
}

func (te TabEditor) updateAddRule(msg tea.KeyMsg) (TabEditor, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		te.mode = temEdit
	case tea.KeyEnter:
		pattern := strings.TrimSpace(te.inputBuf)
		if pattern != "" {
			te.tabs[te.editIdx].Rules = append(te.tabs[te.editIdx].Rules, config.TabRuleConfig{
				Pattern: pattern,
				Include: true,
				Active:  true,
			})
			te.editCur = len(te.tabs[te.editIdx].Rules) - 1
		}
		te.mode = temEdit
	case tea.KeyBackspace:
		if len(te.inputBuf) > 0 {
			te.inputBuf = te.inputBuf[:len(te.inputBuf)-1]
		}
	case tea.KeyRunes:
		te.inputBuf += string(msg.Runes)
	case tea.KeySpace:
		te.inputBuf += " "
	}
	return te, nil
}

func (te TabEditor) updateNewTab(msg tea.KeyMsg) (TabEditor, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		te.mode = temList
	case tea.KeyEnter:
		name := strings.TrimSpace(te.inputBuf)
		if name != "" {
			te.tabs = append(te.tabs, config.CustomTabConfig{
				Name:    name,
				Visible: true,
			})
			te.cursor = len(te.tabs) - 1
			te.editIdx = te.cursor
			te.editCur = 0
			te.mode = temEdit
		} else {
			te.mode = temList
		}
	case tea.KeyBackspace:
		if len(te.inputBuf) > 0 {
			te.inputBuf = te.inputBuf[:len(te.inputBuf)-1]
		}
	case tea.KeyRunes:
		te.inputBuf += string(msg.Runes)
	case tea.KeySpace:
		te.inputBuf += " "
	}
	return te, nil
}

func (te TabEditor) View() string {
	switch te.mode {
	case temList:
		return te.viewList()
	case temEdit, temAddRule:
		return te.viewEdit()
	case temNewTab:
		return te.viewNewTab()
	}
	return ""
}

func (te TabEditor) viewList() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 2).
		Width(50)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Custom Tabs"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
		Render("[Space] show/hide  [D] delete  [Enter] edit  [Esc] save"))
	b.WriteString("\n\n")

	for i, tab := range te.tabs {
		vis := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("○ ")
		if tab.Visible {
			vis = lipgloss.NewStyle().Foreground(colorGreen).Render("● ")
		}

		nameStyle := lipgloss.NewStyle().Foreground(colorDim)
		cursor := "  "
		if i == te.cursor {
			nameStyle = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
			cursor = "> "
		}

		ruleCount := lipgloss.NewStyle().Foreground(colorDim).
			Render("(" + strconv.Itoa(len(tab.Rules)) + " rules)")

		b.WriteString(cursor + vis + nameStyle.Render(tab.Name) + " " + ruleCount)
		b.WriteByte('\n')
	}

	// "Add new tab"
	if te.cursor == len(te.tabs) {
		b.WriteString("> " + lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("+ Add new tab..."))
	} else {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorDim).Render("+ Add new tab..."))
	}
	b.WriteByte('\n')

	return lipgloss.Place(te.width, te.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}

func (te TabEditor) viewEdit() string {
	tab := te.tabs[te.editIdx]
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 2).
		Width(60)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit: " + tab.Name))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
		Render("[Space] enable/disable  [T] toggle match/exclude  [D] delete  [Esc] back"))
	b.WriteString("\n\n")

	for i, rule := range tab.Rules {
		active := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("○ ")
		if rule.Active {
			active = lipgloss.NewStyle().Foreground(colorGreen).Render("● ")
		}

		matchType := lipgloss.NewStyle().Foreground(colorGreen).Render("MATCH  ")
		if !rule.Include {
			matchType = lipgloss.NewStyle().Foreground(colorRed).Render("EXCLUDE")
		}

		patStyle := lipgloss.NewStyle().Foreground(colorDim)
		cursor := "  "
		if i == te.editCur {
			patStyle = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
			cursor = "> "
		}

		b.WriteString(cursor + active + matchType + " " + patStyle.Render(rule.Pattern))
		b.WriteByte('\n')
	}

	// "Add match"
	if te.mode == temAddRule {
		b.WriteString("> " + lipgloss.NewStyle().Foreground(colorOrange).Render("+ Pattern: "+te.inputBuf+"█"))
	} else if te.editCur == len(tab.Rules) {
		b.WriteString("> " + lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("+ Add match..."))
	} else {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorDim).Render("+ Add match..."))
	}
	b.WriteByte('\n')

	return lipgloss.Place(te.width, te.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}

func (te TabEditor) viewNewTab() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 2).
		Width(50)

	var b strings.Builder
	b.WriteString(titleStyle.Render("New Tab"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorOrange).Render("Name: " + te.inputBuf + "█"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render("[Enter] create  [Esc] cancel"))

	return lipgloss.Place(te.width, te.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
