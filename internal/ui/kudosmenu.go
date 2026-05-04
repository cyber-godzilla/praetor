package ui

import (
	"fmt"
	"strings"

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
	case tea.KeyEnter:
		if m.cursor < 0 || m.cursor >= len(m.rows) {
			return m, nil
		}
		row := m.rows[m.cursor]
		if !row.isSelectable() {
			return m, nil
		}
		if row.section == kudosSectionFavorites && row.favIdx >= 0 {
			name := m.kudos.Favorites[row.favIdx]
			return m, m.closeCmd("@kudos "+name, "")
		}
		if row.section == kudosSectionQueue && row.queueIdx >= 0 {
			e := m.kudos.Queue[row.queueIdx]
			return m, m.closeCmd("", "@kudos "+e.Name+" "+e.Message)
		}
		return m, nil
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}
		switch msg.Runes[0] {
		case 'd', 'D':
			return m.handleDelete(), nil
		case 'c', 'C':
			m.editMode = kudosEditAddFavorite
			m.editBuf = ""
			return m, nil
		case 'q', 'Q':
			m.editMode = kudosEditAddQueueName
			m.editBuf = ""
			m.pendingQueueName = ""
			return m, nil
		}
		return m, nil
	}
	return m, nil
}

func (m KudosMenu) handleDelete() KudosMenu {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return m
	}
	row := m.rows[m.cursor]
	if !row.isSelectable() {
		return m
	}
	if row.section == kudosSectionFavorites && row.favIdx >= 0 {
		m.kudos.RemoveFavoriteAt(row.favIdx)
	} else if row.section == kudosSectionQueue && row.queueIdx >= 0 {
		m.kudos.RemoveQueueAt(row.queueIdx)
	}
	m.rebuildRows()
	if m.cursor >= len(m.rows) || !m.rows[m.cursor].isSelectable() {
		found := -1
		for i := m.cursor; i < len(m.rows); i++ {
			if m.rows[i].isSelectable() {
				found = i
				break
			}
		}
		if found == -1 {
			for i := m.cursor - 1; i >= 0; i-- {
				if m.rows[i].isSelectable() {
					found = i
					break
				}
			}
		}
		m.cursor = found
	}
	return m
}

func (m KudosMenu) updateEditing(msg tea.KeyMsg) (KudosMenu, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.editMode = kudosEditNone
		m.editBuf = ""
		m.pendingQueueName = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.editBuf) > 0 {
			r := []rune(m.editBuf)
			m.editBuf = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyEnter:
		return m.commitEdit(), nil
	case tea.KeySpace:
		m.editBuf += " "
		return m, nil
	case tea.KeyRunes:
		m.editBuf += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

func (m KudosMenu) commitEdit() KudosMenu {
	buf := m.editBuf
	switch m.editMode {
	case kudosEditAddFavorite:
		m.kudos.AddFavorite(buf) // dedup + sort handled in config helper
		m.editMode = kudosEditNone
		m.editBuf = ""
		m.rebuildRows()
		m.cursor = m.findFavoriteRow(buf)
		if m.cursor < 0 {
			m.cursor = m.firstSelectable()
		}
	case kudosEditAddQueueName:
		name := strings.TrimSpace(m.editBuf)
		if name == "" {
			m.editMode = kudosEditNone
			m.editBuf = ""
			return m
		}
		m.pendingQueueName = name
		m.editMode = kudosEditAddQueueMessage
		m.editBuf = ""
	case kudosEditAddQueueMessage:
		oldLen := len(m.kudos.Queue)
		m.kudos.AddQueueEntry(m.pendingQueueName, m.editBuf) // helper trims + rejects empties
		name := m.pendingQueueName
		m.editMode = kudosEditNone
		m.editBuf = ""
		m.pendingQueueName = ""
		m.rebuildRows()
		if len(m.kudos.Queue) > oldLen {
			// Entry was added
			m.cursor = m.findLastQueueRowFor(name)
			if m.cursor < 0 {
				m.cursor = m.firstSelectable()
			}
		} else {
			// Entry was rejected (empty message)
			m.cursor = m.firstSelectable()
		}
	}
	return m
}

func (m KudosMenu) findFavoriteRow(name string) int {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return -1
	}
	for i, r := range m.rows {
		if r.section == kudosSectionFavorites && r.favIdx >= 0 {
			if strings.ToLower(m.kudos.Favorites[r.favIdx]) == target {
				return i
			}
		}
	}
	return -1
}

func (m KudosMenu) findLastQueueRowFor(name string) int {
	target := strings.ToLower(strings.TrimSpace(name))
	last := -1
	for i, r := range m.rows {
		if r.section == kudosSectionQueue && r.queueIdx >= 0 {
			if strings.ToLower(m.kudos.Queue[r.queueIdx].Name) == target {
				last = i
			}
		}
	}
	return last
}
