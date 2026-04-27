package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cyber-godzilla/praetor/internal/wiki"
)

// WikiOpenMsg is emitted when the user selects a bookmark to open.
type WikiOpenMsg struct {
	Key  string
	Slug string
}

// WikiMenuCloseMsg is emitted when the user closes the wiki menu.
type WikiMenuCloseMsg struct{}

// MenuWikiMsg is sent to open the wiki menu.
type MenuWikiMsg struct{}

type wikiItem struct {
	isHeader bool
	label    string
	key      string
	slug     string
}

// WikiMenu is a scrollable bookmark browser overlay.
type WikiMenu struct {
	items  []wikiItem
	cursor int
	width  int
	height int
}

// NewWikiMenu constructs the wiki menu from the curated bookmark list.
func NewWikiMenu() WikiMenu {
	var items []wikiItem
	for _, sec := range wiki.Sections() {
		items = append(items, wikiItem{isHeader: true, label: sec.Name})
		for _, bm := range sec.Bookmarks {
			items = append(items, wikiItem{label: bm.Key, key: bm.Key, slug: bm.Slug})
		}
	}
	wm := WikiMenu{items: items}
	for i, it := range items {
		if !it.isHeader {
			wm.cursor = i
			break
		}
	}
	return wm
}

// SetSize updates the terminal dimensions used for rendering.
func (m *WikiMenu) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles key input for the wiki menu.
func (m WikiMenu) Update(msg tea.KeyMsg) (WikiMenu, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		return m, func() tea.Msg { return WikiMenuCloseMsg{} }

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
		if m.cursor >= 0 && m.cursor < len(m.items) {
			it := m.items[m.cursor]
			if !it.isHeader {
				key, slug := it.key, it.slug
				return m, func() tea.Msg { return WikiOpenMsg{Key: key, Slug: slug} }
			}
		}
		return m, nil
	}

	return m, nil
}

// visibleSlice returns the start and end indices of the items window to render.
func (m WikiMenu) visibleSlice() (start, end int) {
	visible := m.height - 8 // title (1) + blank (1) + hint (1) + padding (5)
	if visible < 8 {
		visible = 8
	}
	start = m.cursor - visible/2
	if start < 0 {
		start = 0
	}
	end = start + visible
	if end > len(m.items) {
		end = len(m.items)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}
	return
}

// View renders the wiki menu centered on screen.
func (m WikiMenu) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	cursorStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	boxWidth := m.width - 10
	if boxWidth < 36 {
		boxWidth = 36
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Wiki Bookmarks"))
	b.WriteString("\n\n")

	start, end := m.visibleSlice()

	if start > 0 {
		b.WriteString(dimStyle.Render("  ..."))
		b.WriteByte('\n')
	}

	for i := start; i < end; i++ {
		it := m.items[i]
		if it.isHeader {
			b.WriteString(headerStyle.Render("  " + it.label))
			b.WriteByte('\n')
			continue
		}
		if i == m.cursor {
			b.WriteString(cursorStyle.Render(fmt.Sprintf("  > %s", it.label)))
		} else {
			b.WriteString(dimStyle.Render(fmt.Sprintf("    %s", it.label)))
		}
		b.WriteByte('\n')
	}

	if end < len(m.items) {
		b.WriteString(dimStyle.Render("  ..."))
		b.WriteByte('\n')
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("[↑/↓] navigate  [Enter] open  [Esc] close"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}
