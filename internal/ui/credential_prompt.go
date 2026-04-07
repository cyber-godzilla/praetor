package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CredentialStoreMsg is sent when the user decides whether to store credentials.
type CredentialStoreMsg struct {
	Username string
	Password string
	Store    bool
}

// CredentialPrompt asks the user whether to save login credentials.
type CredentialPrompt struct {
	username      string
	password      string
	alreadyStored bool
	width         int
	height        int
}

// NewCredentialPrompt creates a new prompt for the given username.
func NewCredentialPrompt(username, password string, alreadyStored bool) CredentialPrompt {
	return CredentialPrompt{username: username, password: password, alreadyStored: alreadyStored}
}

// SetSize updates the prompt dimensions.
func (c *CredentialPrompt) SetSize(w, h int) {
	c.width = w
	c.height = h
}

// Update handles key messages for the credential prompt.
func (c CredentialPrompt) Update(msg tea.KeyMsg) (CredentialPrompt, tea.Cmd) {
	if c.alreadyStored {
		// Already stored — any key continues to game.
		return c, func() tea.Msg {
			return CredentialStoreMsg{Username: c.username, Password: c.password, Store: false}
		}
	}
	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'y', 'Y':
				return c, func() tea.Msg {
					return CredentialStoreMsg{Username: c.username, Password: c.password, Store: true}
				}
			case 'n', 'N':
				return c, func() tea.Msg {
					return CredentialStoreMsg{Username: c.username, Password: c.password, Store: false}
				}
			}
		}
	}
	return c, nil
}

// View renders the credential storage prompt.
func (c CredentialPrompt) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(50)

	questionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#cccccc"))
	hintStyle := lipgloss.NewStyle().Foreground(colorDim)

	var content string
	if c.alreadyStored {
		content = titleStyle.Render("Login to The Eternal City") + "\n\n" +
			questionStyle.Render(fmt.Sprintf("Credentials already stored for %q", c.username)) + "\n\n" +
			hintStyle.Render("Press any key to continue")
	} else {
		content = titleStyle.Render("Login to The Eternal City") + "\n\n" +
			questionStyle.Render(fmt.Sprintf("Store credentials for %q?", c.username)) + "\n\n" +
			hintStyle.Render("[Y] Yes   [N] No")
	}

	box := boxStyle.Render(content)
	return lipgloss.Place(c.width, c.height, lipgloss.Center, lipgloss.Center, box)
}
