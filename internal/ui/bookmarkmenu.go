package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BookmarkSection is a named group of (key, slug) entries used by
// BookmarkMenu. Order is preserved.
type BookmarkSection struct {
	Name      string
	Bookmarks []BookmarkItem
}

// BookmarkItem is a single (key, slug) entry.
type BookmarkItem struct {
	Key  string
	Slug string
}

// BookmarkMenu is a generic, scrollable, sectioned list overlay used by
// /wiki and /maps (and any future similar menus). Callers configure the
// title, the data, and the messages emitted on Enter / Esc.
type BookmarkMenu struct {
	title   string
	items   []bookmarkRow
	cursor  int
	width   int
	height  int
	onOpen  func(key, slug string) tea.Msg
	onClose func() tea.Msg
}

type bookmarkRow struct {
	isHeader bool
	label    string
	key      string
	slug     string
}

// NewBookmarkMenu constructs a menu from sectioned data. onOpen is fired
// when the user presses Enter on a non-header row; onClose fires on Esc.
func NewBookmarkMenu(title string, sections []BookmarkSection, onOpen func(key, slug string) tea.Msg, onClose func() tea.Msg) BookmarkMenu {
	var items []bookmarkRow
	for _, sec := range sections {
		items = append(items, bookmarkRow{isHeader: true, label: sec.Name})
		for _, bm := range sec.Bookmarks {
			items = append(items, bookmarkRow{label: bm.Key, key: bm.Key, slug: bm.Slug})
		}
	}
	bm := BookmarkMenu{
		title:   title,
		items:   items,
		onOpen:  onOpen,
		onClose: onClose,
	}
	for i, it := range items {
		if !it.isHeader {
			bm.cursor = i
			break
		}
	}
	return bm
}

// SetSize updates the terminal dimensions used for rendering.
func (m *BookmarkMenu) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles key input for the bookmark menu.
func (m BookmarkMenu) Update(msg tea.KeyMsg) (BookmarkMenu, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		if m.onClose != nil {
			return m, func() tea.Msg { return m.onClose() }
		}
		return m, nil

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
			if !it.isHeader && m.onOpen != nil {
				key, slug := it.key, it.slug
				return m, func() tea.Msg { return m.onOpen(key, slug) }
			}
		}
		return m, nil
	}

	return m, nil
}

// View renders the bookmark menu centered on screen.
func (m BookmarkMenu) View() string {
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
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	maxVisible := m.height - 8 // title (1) + blank (1) + hint (1) + padding (5)
	if maxVisible < 8 {
		maxVisible = 8
	}
	start, end := viewportWindowCentered(len(m.items), maxVisible, m.cursor)

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
