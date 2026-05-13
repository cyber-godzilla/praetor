package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InputSubmitMsg is sent when the user presses Enter in the input bar.
type InputSubmitMsg struct {
	Value string
}

// InputSetValueMsg, when handled by App, replaces the input text and
// places the cursor at end. Used by the Kudos menu to pre-fill
// "@kudos <name>" after the user picks a Favorite.
type InputSetValueMsg struct {
	Value string
}

// Input is the text input bar with command history.
type Input struct {
	textinput textinput.Model
	history   []string
	histIdx   int // -1 = not browsing history
	width     int

	// Pre-rendered top-border line, regenerated only when width changes.
	// The full lipgloss border render (inputStyle.Width(w).Render(...))
	// dominated CPU in profiling because it measures content cell-width
	// on every frame; precomputing the width-only piece lets View skip
	// that machinery for ~every keystroke / blink frame.
	cachedBorder      string
	cachedBorderWidth int
}

// NewInput creates a new Input component.
func NewInput() Input {
	ti := textinput.New()
	ti.Prompt = promptStyle.Render("> ")
	ti.Focus()
	ti.CharLimit = 256

	return Input{
		textinput: ti,
		histIdx:   -1,
	}
}

// Init returns the text input's init command (cursor blink).
func (i Input) Init() tea.Cmd {
	return textinput.Blink
}

// SetWidth updates the input width.
func (i *Input) SetWidth(w int) {
	i.width = w
	i.textinput.Width = w - 4 // Account for prompt and padding
	i.refreshBorder()
}

// refreshBorder rebuilds the cached border line for the current width.
// Called from SetWidth — width changes are rare so the cost is amortized
// across many View frames.
func (i *Input) refreshBorder() {
	if i.width <= 0 {
		i.cachedBorder = ""
		i.cachedBorderWidth = 0
		return
	}
	i.cachedBorder = lipgloss.NewStyle().
		Foreground(colorBorder).
		Render(strings.Repeat("─", i.width))
	i.cachedBorderWidth = i.width
}

// Update handles key messages for the input bar.
func (i Input) Update(msg tea.Msg) (Input, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := i.textinput.Value()
			// Add non-empty commands to history.
			if val != "" {
				i.history = append(i.history, val)
				i.histIdx = -1
			}
			i.textinput.SetValue("")
			return i, func() tea.Msg {
				return InputSubmitMsg{Value: val}
			}

		case tea.KeyUp:
			if len(i.history) == 0 {
				return i, nil
			}
			if i.histIdx == -1 {
				i.histIdx = len(i.history) - 1
			} else if i.histIdx > 0 {
				i.histIdx--
			}
			i.textinput.SetValue(i.history[i.histIdx])
			i.textinput.CursorEnd()
			return i, nil

		case tea.KeyDown:
			if i.histIdx == -1 {
				return i, nil
			}
			if i.histIdx < len(i.history)-1 {
				i.histIdx++
				i.textinput.SetValue(i.history[i.histIdx])
				i.textinput.CursorEnd()
			} else {
				i.histIdx = -1
				i.textinput.SetValue("")
			}
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.textinput, cmd = i.textinput.Update(msg)
	return i, cmd
}

// View renders the input bar.
//
// We hand-roll what inputStyle.Width(w).Render(content) does (a single
// width-w border line, newline, content padded to width) so we can skip
// the lipgloss border/padding pipeline that measures cell-widths on
// every styled rune. The textinput's own content still renders fresh
// each frame — its cursor blink and value change frame-to-frame — but
// the surrounding chrome is now constant work per width change.
func (i Input) View() string {
	if i.cachedBorder == "" || i.cachedBorderWidth != i.width {
		// Width hasn't been set yet (or was zero) — fall back to the
		// full lipgloss path so a fresh component still renders.
		return inputStyle.Width(i.width).Render(i.textinput.View())
	}
	content := i.textinput.View()
	padding := i.width - visibleWidth(content)
	if padding > 0 {
		var b strings.Builder
		b.Grow(len(i.cachedBorder) + 1 + len(content) + padding)
		b.WriteString(i.cachedBorder)
		b.WriteByte('\n')
		b.WriteString(content)
		b.WriteString(strings.Repeat(" ", padding))
		return b.String()
	}
	return i.cachedBorder + "\n" + content
}

// Focus gives focus to the text input.
func (i *Input) Focus() tea.Cmd {
	return i.textinput.Focus()
}

// Blur removes focus from the text input.
func (i *Input) Blur() {
	i.textinput.Blur()
}

// Focused returns whether the input is focused.
func (i Input) Focused() bool {
	return i.textinput.Focused()
}
