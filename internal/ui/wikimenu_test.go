package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWikiMenu_PopulatesItems(t *testing.T) {
	m := NewWikiMenu()
	if len(m.items) == 0 {
		t.Fatal("NewWikiMenu returned a menu with no items")
	}
}

func TestWikiMenu_InitialCursorOnBookmark(t *testing.T) {
	m := NewWikiMenu()
	if m.cursor < 0 || m.cursor >= len(m.items) {
		t.Fatalf("cursor %d out of range", m.cursor)
	}
	if m.items[m.cursor].isHeader {
		t.Error("initial cursor should be on a bookmark, not a header")
	}
}

func TestWikiMenu_EnterProducesWikiOpenMsg(t *testing.T) {
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

func TestWikiMenu_EscProducesWikiMenuCloseMsg(t *testing.T) {
	m := NewWikiMenu()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should produce a command")
	}
	if _, ok := cmd().(WikiMenuCloseMsg); !ok {
		t.Errorf("expected WikiMenuCloseMsg, got %T", cmd())
	}
}
