package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWikiMenu_StartsOnFirstBookmark(t *testing.T) {
	m := NewWikiMenu()
	if m.cursor < 0 || m.cursor >= len(m.items) {
		t.Fatalf("cursor %d out of range", m.cursor)
	}
	if m.items[m.cursor].isHeader {
		t.Error("initial cursor should be on a bookmark, not a header")
	}
}

func TestWikiMenu_DownSkipsHeaders(t *testing.T) {
	m := NewWikiMenu()
	start := m.cursor
	for i := 0; i < 10; i++ {
		var c tea.Cmd
		m, c = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		_ = c
		if m.items[m.cursor].isHeader {
			t.Errorf("cursor landed on a header after Down (i=%d, label=%q)", i, m.items[m.cursor].label)
		}
	}
	if m.cursor == start {
		t.Error("cursor did not move on Down")
	}
}

func TestWikiMenu_EnterEmitsOpenMsg(t *testing.T) {
	m := NewWikiMenu()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on bookmark should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(WikiOpenMsg); !ok {
		t.Errorf("expected WikiOpenMsg, got %T", msg)
	}
}

func TestWikiMenu_EscapeEmitsCloseMsg(t *testing.T) {
	m := NewWikiMenu()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should produce a command")
	}
	if _, ok := cmd().(WikiMenuCloseMsg); !ok {
		t.Errorf("expected WikiMenuCloseMsg, got %T", cmd())
	}
}
