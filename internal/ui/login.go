package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoginSubmitMsg is sent when the user submits the login form.
type LoginSubmitMsg struct {
	Username string
	Password string
}

// LoginScreen renders a centered login form with username and password fields.
type LoginScreen struct {
	username    textinput.Model
	password    textinput.Model
	focused     int // 0=username, 1=password
	err         string
	width       int
	height      int
	hasAccounts bool // true if stored accounts exist (enables Esc to go back)
}

// NewLoginScreen creates a new LoginScreen.
func NewLoginScreen() LoginScreen {
	user := textinput.New()
	user.Placeholder = "Username"
	user.Focus()
	user.CharLimit = 64
	user.Width = 30

	pass := textinput.New()
	pass.Placeholder = "Password"
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'
	pass.CharLimit = 64
	pass.Width = 30

	return LoginScreen{
		username: user,
		password: pass,
		focused:  0,
	}
}

// Init returns the initial command (cursor blink for focused field).
func (l LoginScreen) Init() tea.Cmd {
	return textinput.Blink
}

// SetSize updates the login screen dimensions.
func (l *LoginScreen) SetSize(w, h int) {
	l.width = w
	l.height = h
}

// SetError sets an error message to display.
func (l *LoginScreen) SetError(err string) {
	l.err = err
}

// Update handles key messages for the login screen.
func (l LoginScreen) Update(msg tea.Msg) (LoginScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab, tea.KeyShiftTab:
			// Toggle focus between fields
			if l.focused == 0 {
				l.focused = 1
				l.username.Blur()
				return l, l.password.Focus()
			}
			l.focused = 0
			l.password.Blur()
			return l, l.username.Focus()

		case tea.KeyEnter:
			if l.focused == 0 {
				// Move to password
				l.focused = 1
				l.username.Blur()
				return l, l.password.Focus()
			}
			// Submit
			user := l.username.Value()
			pass := l.password.Value()
			if user == "" || pass == "" {
				l.err = "Username and password required"
				return l, nil
			}
			return l, func() tea.Msg {
				return LoginSubmitMsg{Username: user, Password: pass}
			}
		}
	}

	// Forward to focused input
	var cmd tea.Cmd
	if l.focused == 0 {
		l.username, cmd = l.username.Update(msg)
	} else {
		l.password, cmd = l.password.Update(msg)
	}
	return l, cmd
}

// View renders the centered login form.
func (l LoginScreen) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(40)

	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	errStyle := lipgloss.NewStyle().Foreground(colorRed)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Login to The Eternal City"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Username:"))
	b.WriteByte('\n')
	b.WriteString(l.username.View())
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Password:"))
	b.WriteByte('\n')
	b.WriteString(l.password.View())
	b.WriteByte('\n')

	if l.err != "" {
		b.WriteByte('\n')
		b.WriteString(errStyle.Render(l.err))
	}

	b.WriteString("\n\n")
	hint := "[Tab] switch fields  [Enter] submit"
	if l.hasAccounts {
		hint += "  [Esc] back"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).Render(hint))

	box := boxStyle.Render(b.String())

	// Center the box in the terminal
	return lipgloss.Place(l.width, l.height, lipgloss.Center, lipgloss.Center, box)
}
