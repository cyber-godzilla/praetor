package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cyber-godzilla/praetor/internal/wiki"
)

// MapsOpenMsg is emitted when the user picks a map from the menu.
type MapsOpenMsg struct {
	Key  string
	Slug string
}

// MapsMenuCloseMsg is emitted when the user dismisses the maps menu.
type MapsMenuCloseMsg struct{}

// MenuMapsMsg requests opening the maps menu (analogous to MenuWikiMsg).
type MenuMapsMsg struct{}

// NewMapsMenu returns a BookmarkMenu wired to the wiki package's
// curated map list.
func NewMapsMenu() BookmarkMenu {
	sections := make([]BookmarkSection, 0, len(wiki.MapSections()))
	for _, sec := range wiki.MapSections() {
		bms := make([]BookmarkItem, 0, len(sec.Bookmarks))
		for _, bm := range sec.Bookmarks {
			bms = append(bms, BookmarkItem{Key: bm.Key, Slug: bm.Slug})
		}
		sections = append(sections, BookmarkSection{Name: sec.Name, Bookmarks: bms})
	}
	return NewBookmarkMenu(
		"Maps",
		sections,
		func(key, slug string) tea.Msg { return MapsOpenMsg{Key: key, Slug: slug} },
		func() tea.Msg { return MapsMenuCloseMsg{} },
	)
}
