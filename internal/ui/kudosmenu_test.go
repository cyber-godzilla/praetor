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

func TestKudosMenu_EnterOnFavoritePrefillsInput(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{Favorites: []string{"Alice"}})
	if m.rows[m.cursor].label != "Alice" {
		t.Fatalf("expected cursor on Alice, got %q", m.rows[m.cursor].label)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected close cmd")
	}
	cm := cmd().(KudosCloseMsg)
	if cm.Prefill != "@kudos Alice" {
		t.Errorf("Prefill=%q, want %q", cm.Prefill, "@kudos Alice")
	}
	if cm.Send != "" {
		t.Errorf("Send should be empty, got %q", cm.Send)
	}
	if len(cm.Kudos.Favorites) != 1 {
		t.Errorf("favorite should still exist after Enter: %+v", cm.Kudos.Favorites)
	}
}

func TestKudosMenu_EnterOnQueueSendsCommand(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{
		Queue: []config.KudosQueueEntry{{Name: "Bob", Message: "thanks for the rescue"}},
	})
	if m.rows[m.cursor].section != kudosSectionQueue {
		t.Fatalf("expected cursor in queue, got %v", m.rows[m.cursor].section)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cm := cmd().(KudosCloseMsg)
	if cm.Send != "@kudos Bob thanks for the rescue" {
		t.Errorf("Send=%q", cm.Send)
	}
	if cm.Prefill != "" {
		t.Errorf("Prefill should be empty, got %q", cm.Prefill)
	}
	if len(cm.Kudos.Queue) != 1 {
		t.Errorf("queue entry should still exist after Enter: %+v", cm.Kudos.Queue)
	}
}

func TestKudosMenu_DeleteFavorite(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{Favorites: []string{"Alice", "Bjorn"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if len(m.kudos.Favorites) != 1 || m.kudos.Favorites[0] != "Bjorn" {
		t.Errorf("after delete: %v", m.kudos.Favorites)
	}
	if m.rows[m.cursor].label != "Bjorn" {
		t.Errorf("cursor not on Bjorn: %+v", m.rows[m.cursor])
	}
}

func TestKudosMenu_DeleteQueueEntry(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{
		Queue: []config.KudosQueueEntry{
			{Name: "Bob", Message: "thanks"},
			{Name: "Cara", Message: "great rp"},
		},
	})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if len(m.kudos.Queue) != 1 || m.kudos.Queue[0].Name != "Cara" {
		t.Errorf("after delete: %+v", m.kudos.Queue)
	}
}

func TestKudosMenu_DeleteUppercaseD(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{Favorites: []string{"Alice"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	if len(m.kudos.Favorites) != 0 {
		t.Errorf("uppercase D should also delete: %v", m.kudos.Favorites)
	}
}

func TestKudosMenu_DeleteOnHintRowIsNoop(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{}) // both sections empty -> hints only
	if m.cursor != -1 {
		t.Fatalf("expected cursor=-1 on all-empty menu, got %d", m.cursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.cursor != -1 {
		t.Errorf("cursor changed: %d", m.cursor)
	}
}

func TestKudosMenu_AddFavoriteFlow(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.editMode != kudosEditAddFavorite {
		t.Fatalf("editMode=%v", m.editMode)
	}
	for _, r := range "Alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editMode != kudosEditNone {
		t.Errorf("expected editMode=none after commit, got %v", m.editMode)
	}
	if len(m.kudos.Favorites) != 1 || m.kudos.Favorites[0] != "Alice" {
		t.Errorf("favorites=%v", m.kudos.Favorites)
	}
}

func TestKudosMenu_AddFavoriteEscCancels(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	for _, r := range "Alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.editMode != kudosEditNone {
		t.Errorf("editMode should reset")
	}
	if len(m.kudos.Favorites) != 0 {
		t.Errorf("Esc should cancel: %v", m.kudos.Favorites)
	}
}

func TestKudosMenu_AddFavoriteEmptyIsNoop(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editMode != kudosEditNone {
		t.Errorf("editMode should reset")
	}
	if len(m.kudos.Favorites) != 0 {
		t.Errorf("favorites should remain empty: %v", m.kudos.Favorites)
	}
}

func TestKudosMenu_AddFavoriteDuplicate(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{Favorites: []string{"Alice"}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	for _, r := range "alice" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.kudos.Favorites) != 1 || m.kudos.Favorites[0] != "Alice" {
		t.Errorf("dedup failed: %v", m.kudos.Favorites)
	}
}

func TestKudosMenu_AddFavoriteBackspace(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	for _, r := range "Alixe" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	for _, r := range "ce" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.kudos.Favorites) != 1 || m.kudos.Favorites[0] != "Alice" {
		t.Errorf("after backspace: %v", m.kudos.Favorites)
	}
}

func TestKudosMenu_AddQueueFlow(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.editMode != kudosEditAddQueueName {
		t.Fatalf("editMode=%v after q", m.editMode)
	}
	for _, r := range "Bob" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editMode != kudosEditAddQueueMessage {
		t.Fatalf("expected message-prompt, got %v", m.editMode)
	}
	if m.pendingQueueName != "Bob" {
		t.Errorf("pendingQueueName=%q", m.pendingQueueName)
	}
	for _, r := range "thanks" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	for _, r := range "lots" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.editMode != kudosEditNone {
		t.Errorf("editMode should reset")
	}
	if len(m.kudos.Queue) != 1 || m.kudos.Queue[0].Name != "Bob" || m.kudos.Queue[0].Message != "thanks lots" {
		t.Errorf("queue=%+v", m.kudos.Queue)
	}
}

func TestKudosMenu_AddQueueEscAtNameCancels(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	for _, r := range "Bob" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.editMode != kudosEditNone || m.pendingQueueName != "" {
		t.Errorf("Esc should fully reset edit state")
	}
	if len(m.kudos.Queue) != 0 {
		t.Errorf("queue should be empty: %v", m.kudos.Queue)
	}
}

func TestKudosMenu_AddQueueEscAtMessageCancels(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	for _, r := range "Bob" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "thanks" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if len(m.kudos.Queue) != 0 {
		t.Errorf("Esc at message step should cancel: %v", m.kudos.Queue)
	}
	if m.editMode != kudosEditNone || m.pendingQueueName != "" {
		t.Errorf("edit state not fully reset")
	}
}

func TestKudosMenu_AddQueueEmptyMessageIsNoop(t *testing.T) {
	m := NewKudosMenu(config.KudosConfig{})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	for _, r := range "Bob" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.kudos.Queue) != 0 {
		t.Errorf("empty message should not queue: %v", m.kudos.Queue)
	}
	if m.editMode != kudosEditNone {
		t.Errorf("editMode should reset")
	}
}
