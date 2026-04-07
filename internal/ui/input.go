package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// InputSubmitMsg is sent when the user presses Enter in the input bar.
type InputSubmitMsg struct {
	Value string
}

// Input is the text input bar with command history.
type Input struct {
	textinput textinput.Model
	history   []string
	histIdx   int // -1 = not browsing history
	width     int
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
func (i Input) View() string {
	return inputStyle.Width(i.width).Render(i.textinput.View())
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
