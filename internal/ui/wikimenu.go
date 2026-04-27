package ui

import (
	tea "github.com/charmbracelet/bubbletea"

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

// NewWikiMenu returns a BookmarkMenu wired to the wiki package's
// curated bookmark list.
func NewWikiMenu() BookmarkMenu {
	sections := make([]BookmarkSection, 0, len(wiki.Sections()))
	for _, sec := range wiki.Sections() {
		bms := make([]BookmarkItem, 0, len(sec.Bookmarks))
		for _, bm := range sec.Bookmarks {
			bms = append(bms, BookmarkItem{Key: bm.Key, Slug: bm.Slug})
		}
		sections = append(sections, BookmarkSection{Name: sec.Name, Bookmarks: bms})
	}
	return NewBookmarkMenu(
		"Wiki Bookmarks",
		sections,
		func(key, slug string) tea.Msg { return WikiOpenMsg{Key: key, Slug: slug} },
		func() tea.Msg { return WikiMenuCloseMsg{} },
	)
}
