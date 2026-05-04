package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestKudosMenu_NavigationSkipsHeadersAndHints(t *testing.T) {
	// Empty kudos: both sections render their hint row.
	m := NewKudosMenu(config.KudosConfig{})

	if m.cursor != -1 {
		t.Errorf("expected cursor=-1 with all-empty kudos, got %d", m.cursor)
	}

	m = NewKudosMenu(config.KudosConfig{
		Favorites: []string{"Alice"},
		Queue:     []config.KudosQueueEntry{{Name: "Bob", Message: "thanks"}},
	})
	if m.cursor < 0 || !m.rows[m.cursor].isSelectable() {
		t.Fatalf("initial cursor=%d not selectable: %+v", m.cursor, m.rows[m.cursor])
	}
	if m.rows[m.cursor].section != kudosSectionFavorites {
		t.Errorf("expected initial cursor in favorites, got section %v", m.rows[m.cursor].section)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.rows[m.cursor].section != kudosSectionQueue {
		t.Errorf("expected cursor in queue after down, got section %v", m.rows[m.cursor].section)
	}

	prev := m.cursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != prev {
		t.Errorf("expected cursor to stay at %d, got %d", prev, m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.rows[m.cursor].section != kudosSectionFavorites {
		t.Errorf("expected cursor back in favorites, got %v", m.rows[m.cursor].section)
	}
}

func TestKudosMenu_EscEmitsCloseMsg(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{Favorites: []string{"Alice"}})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	cm, ok := msg.(KudosCloseMsg)
	if !ok {
		t.Fatalf("expected KudosCloseMsg, got %T", msg)
	}
	if len(cm.Kudos.Favorites) != 1 || cm.Kudos.Favorites[0] != "Alice" {
		t.Errorf("Esc lost state: %+v", cm.Kudos)
	}
	if cm.Prefill != "" || cm.Send != "" {
		t.Errorf("Esc should produce empty Prefill/Send, got %+v", cm)
	}
}
