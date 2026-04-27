package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMapsMenu_PopulatesItems(t *testing.T) {
	m := NewMapsMenu()
	if len(m.items) == 0 {
		t.Fatal("NewMapsMenu returned a menu with no items")
	}
}

func TestMapsMenu_InitialCursorOnBookmark(t *testing.T) {
	m := NewMapsMenu()
	if m.cursor < 0 || m.cursor >= len(m.items) {
		t.Fatalf("cursor %d out of range", m.cursor)
	}
	if m.items[m.cursor].isHeader {
		t.Error("initial cursor should be on a bookmark, not a header")
	}
}

func TestMapsMenu_EnterProducesMapsOpenMsg(t *testing.T) {
	m := NewMapsMenu()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on bookmark should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(MapsOpenMsg); !ok {
		t.Errorf("expected MapsOpenMsg, got %T", msg)
	}
}

func TestMapsMenu_EscProducesMapsMenuCloseMsg(t *testing.T) {
	m := NewMapsMenu()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should produce a command")
	}
	if _, ok := cmd().(MapsMenuCloseMsg); !ok {
		t.Errorf("expected MapsMenuCloseMsg, got %T", cmd())
	}
}
