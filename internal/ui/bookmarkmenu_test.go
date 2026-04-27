package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testSections returns a simple two-section fixture for BookmarkMenu tests.
func testSections() []BookmarkSection {
	return []BookmarkSection{
		{
			Name: "Section A",
			Bookmarks: []BookmarkItem{
				{Key: "alpha", Slug: "alpha-slug"},
				{Key: "beta", Slug: "beta-slug"},
			},
		},
		{
			Name: "Section B",
			Bookmarks: []BookmarkItem{
				{Key: "gamma", Slug: "gamma-slug"},
			},
		},
	}
}

func newTestMenu() BookmarkMenu {
	return NewBookmarkMenu(
		"Test Menu",
		testSections(),
		func(key, slug string) tea.Msg { return WikiOpenMsg{Key: key, Slug: slug} },
		func() tea.Msg { return WikiMenuCloseMsg{} },
	)
}

func TestBookmarkMenu_CursorStartsOnFirstNonHeader(t *testing.T) {
	m := newTestMenu()
	if m.cursor < 0 || m.cursor >= len(m.items) {
		t.Fatalf("cursor %d out of range [0, %d)", m.cursor, len(m.items))
	}
	if m.items[m.cursor].isHeader {
		t.Errorf("initial cursor is on a header (index %d, label %q)", m.cursor, m.items[m.cursor].label)
	}
}

func TestBookmarkMenu_DownSkipsHeaders(t *testing.T) {
	m := newTestMenu()
	start := m.cursor
	for i := 0; i < 5; i++ {
		var c tea.Cmd
		m, c = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		_ = c
		if m.items[m.cursor].isHeader {
			t.Errorf("cursor landed on a header after Down (i=%d, label=%q)", i, m.items[m.cursor].label)
		}
	}
	// Cursor should have moved at least once (fixture has 3 bookmarks across 2 sections).
	if m.cursor == start {
		t.Error("cursor did not move after Down")
	}
}

func TestBookmarkMenu_UpSkipsHeaders(t *testing.T) {
	m := newTestMenu()
	// Move to last item first.
	for i := 0; i < 10; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	last := m.cursor
	// Now move back up — must not land on a header.
	for i := 0; i < 5; i++ {
		var c tea.Cmd
		m, c = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		_ = c
		if m.items[m.cursor].isHeader {
			t.Errorf("cursor landed on a header after Up (i=%d, label=%q)", i, m.items[m.cursor].label)
		}
	}
	if m.cursor == last {
		t.Error("cursor did not move after Up")
	}
}

func TestBookmarkMenu_EnterFiresOnOpen(t *testing.T) {
	var gotKey, gotSlug string
	m := NewBookmarkMenu(
		"Test",
		testSections(),
		func(key, slug string) tea.Msg {
			gotKey = key
			gotSlug = slug
			return struct{}{}
		},
		func() tea.Msg { return struct{}{} },
	)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	cmd() // invoke to trigger the callback
	if gotKey == "" {
		t.Error("onOpen was not called on Enter")
	}
	it := m.items[m.cursor]
	if gotKey != it.key || gotSlug != it.slug {
		t.Errorf("onOpen called with (%q, %q), want (%q, %q)", gotKey, gotSlug, it.key, it.slug)
	}
}

func TestBookmarkMenu_EscFiresOnClose(t *testing.T) {
	closed := false
	m := NewBookmarkMenu(
		"Test",
		testSections(),
		func(key, slug string) tea.Msg { return struct{}{} },
		func() tea.Msg {
			closed = true
			return struct{}{}
		},
	)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}
	cmd() // invoke to trigger the callback
	if !closed {
		t.Error("onClose was not called on Esc")
	}
}

func TestBookmarkMenu_EmptySectionsNoCrash(t *testing.T) {
	m := NewBookmarkMenu("Empty", nil, nil, nil)
	if len(m.items) != 0 {
		t.Errorf("expected 0 items, got %d", len(m.items))
	}
	// Key operations on an empty menu must not panic.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m.SetSize(80, 24)
	_ = m.View()
}
