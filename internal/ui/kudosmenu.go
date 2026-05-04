package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/config"
)

// KudosCloseMsg is emitted by KudosMenu when the user closes the overlay,
// selects a Favorite (Prefill non-empty), or selects a Queue entry (Send
// non-empty). Kudos always carries the menu's edited copy of state.
type KudosCloseMsg struct {
	Kudos   config.KudosConfig
	Prefill string
	Send    string
}

type kudosSection int

const (
	kudosSectionFavorites kudosSection = iota
	kudosSectionQueue
)

type kudosEditMode int

const (
	kudosEditNone kudosEditMode = iota
	kudosEditAddFavorite
	kudosEditAddQueueName
	kudosEditAddQueueMessage
)

type kudosRow struct {
	isHeader bool
	isHint   bool
	isBlank  bool
	section  kudosSection
	favIdx   int // -1 unless this row represents a favorite
	queueIdx int // -1 unless this row represents a queue entry
	label    string
}

func (r kudosRow) isSelectable() bool {
	return !r.isHeader && !r.isHint && !r.isBlank
}

// KudosMenu is the editable two-section overlay for managing Kudos.
type KudosMenu struct {
	kudos            config.KudosConfig
	rows             []kudosRow
	cursor           int // -1 if no selectable row
	width            int
	height           int
	editMode         kudosEditMode
	editBuf          string
	pendingQueueName string // populated during the q two-step flow
}

// NewKudosMenu builds a menu from a snapshot of KudosConfig. The menu
// edits an internal copy; mutations propagate via KudosCloseMsg.
func NewKudosMenu(k config.KudosConfig) KudosMenu {
	favs := append([]string(nil), k.Favorites...)
	queue := append([]config.KudosQueueEntry(nil), k.Queue...)
	m := KudosMenu{
		kudos:  config.KudosConfig{Favorites: favs, Queue: queue},
		cursor: -1,
	}
	m.rebuildRows()
	m.cursor = m.firstSelectable()
	return m
}

// SetSize updates the viewport dimensions.
func (m *KudosMenu) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *KudosMenu) rebuildRows() {
	m.rows = m.rows[:0]
	m.rows = append(m.rows, kudosRow{isHeader: true, section: kudosSectionFavorites, label: "Kudos Favorites"})
	if len(m.kudos.Favorites) == 0 {
		m.rows = append(m.rows, kudosRow{isHint: true, section: kudosSectionFavorites, label: "(none — press C to add a favorite)"})
	} else {
		for i, name := range m.kudos.Favorites {
			m.rows = append(m.rows, kudosRow{section: kudosSectionFavorites, favIdx: i, queueIdx: -1, label: name})
		}
	}
	m.rows = append(m.rows, kudosRow{isBlank: true})
	m.rows = append(m.rows, kudosRow{isHeader: true, section: kudosSectionQueue, label: "Kudos Queue"})
	if len(m.kudos.Queue) == 0 {
		m.rows = append(m.rows, kudosRow{isHint: true, section: kudosSectionQueue, label: "(none — press Q to add a queue entry)"})
	} else {
		for i, e := range m.kudos.Queue {
			m.rows = append(m.rows, kudosRow{section: kudosSectionQueue, favIdx: -1, queueIdx: i, label: fmt.Sprintf("%s — %s", e.Name, e.Message)})
		}
	}
}

func (m *KudosMenu) firstSelectable() int {
	for i, r := range m.rows {
		if r.isSelectable() {
			return i
		}
	}
	return -1
}

func (m *KudosMenu) closeCmd(prefill, send string) tea.Cmd {
	out := KudosCloseMsg{Kudos: m.kudos, Prefill: prefill, Send: send}
	return func() tea.Msg { return out }
}

// Update handles key input.
func (m KudosMenu) Update(msg tea.KeyMsg) (KudosMenu, tea.Cmd) {
	if m.editMode != kudosEditNone {
		return m.updateEditing(msg)
	}
	switch msg.Type {
	case tea.KeyEscape:
		return m, m.closeCmd("", "")
	case tea.KeyUp:
		for i := m.cursor - 1; i >= 0; i-- {
			if m.rows[i].isSelectable() {
				m.cursor = i
				break
			}
		}
		return m, nil
	case tea.KeyDown:
		for i := m.cursor + 1; i < len(m.rows); i++ {
			if m.rows[i].isSelectable() {
				m.cursor = i
				break
			}
		}
		return m, nil
	}
	return m, nil
}

// updateEditing is a stub; real behavior added in Task 6/7.
func (m KudosMenu) updateEditing(msg tea.KeyMsg) (KudosMenu, tea.Cmd) {
	if msg.Type == tea.KeyEscape {
		m.editMode = kudosEditNone
		m.editBuf = ""
		m.pendingQueueName = ""
	}
	return m, nil
}
