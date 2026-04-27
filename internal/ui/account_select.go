package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AccountSelectMsg is emitted when the user selects a stored account.
type AccountSelectMsg struct {
	Username string
}

// AddAccountMsg is emitted when the user chooses "Add Account".
type AddAccountMsg struct{}

// DeleteAccountMsg is emitted when the user presses 'd' to delete an account.
type DeleteAccountMsg struct {
	Username string
}

// AccountSelect is a Bubbletea component that displays a list of stored
// accounts with cursor navigation. The last item is always "Add Account".
type AccountSelect struct {
	accounts []string
	cursor   int
	width    int
	height   int
}

// NewAccountSelect creates a new AccountSelect with the given sorted usernames.
func NewAccountSelect(accounts []string) AccountSelect {
	return AccountSelect{
		accounts: accounts,
		cursor:   0,
	}
}

// SetSize updates the component dimensions.
func (a *AccountSelect) SetSize(w, h int) {
	a.width = w
	a.height = h
}

// SetAccounts replaces the account list and resets the cursor.
func (a *AccountSelect) SetAccounts(accounts []string) {
	a.accounts = accounts
	if a.cursor >= a.totalItems() {
		a.cursor = a.totalItems() - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// totalItems returns the number of items including "Add Account".
func (a AccountSelect) totalItems() int {
	return len(a.accounts) + 1
}

// isAddAccount returns true if the cursor is on "Add Account".
func (a AccountSelect) isAddAccount() bool {
	return a.cursor == len(a.accounts)
}

// Init returns nil (no initial command needed).
func (a AccountSelect) Init() tea.Cmd {
	return nil
}

// Update handles key messages for the account selection screen.
func (a AccountSelect) Update(msg tea.Msg) (AccountSelect, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if a.cursor > 0 {
				a.cursor--
			}
			return a, nil

		case tea.KeyDown:
			if a.cursor < a.totalItems()-1 {
				a.cursor++
			}
			return a, nil

		case tea.KeyEnter:
			if a.isAddAccount() {
				return a, func() tea.Msg { return AddAccountMsg{} }
			}
			username := a.accounts[a.cursor]
			return a, func() tea.Msg { return AccountSelectMsg{Username: username} }

		case tea.KeyRunes:
			if len(msg.Runes) == 1 && msg.Runes[0] == 'd' {
				// Delete selected account (only if it's an actual account, not "Add Account").
				if !a.isAddAccount() && len(a.accounts) > 0 {
					username := a.accounts[a.cursor]
					return a, func() tea.Msg { return DeleteAccountMsg{Username: username} }
				}
			}
		}
	}
	return a, nil
}

// View renders the account selection screen as a centered box.
func (a AccountSelect) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(colorOrange).
		Bold(true).
		Align(lipgloss.Center)

	boxWidth := a.width - 10
	if boxWidth < 36 {
		boxWidth = 36
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	selectedStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorDim)
	separatorStyle := lipgloss.NewStyle().Foreground(colorDim)
	hintStyle := lipgloss.NewStyle().Foreground(colorDim)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Login to The Eternal City"))
	b.WriteString("\n\n")

	// Render account list.
	for i, name := range a.accounts {
		if i == a.cursor {
			b.WriteString(selectedStyle.Render("  > " + name))
		} else {
			b.WriteString(normalStyle.Render("    " + name))
		}
		b.WriteByte('\n')
	}

	// Separator line.
	b.WriteString(separatorStyle.Render("    " + strings.Repeat("\u2500", 30)))
	b.WriteByte('\n')

	// "Login with a different account" option.
	addLabel := "Add Account"
	if len(a.accounts) > 0 {
		addLabel = "Login with a different account"
	}
	if a.isAddAccount() {
		b.WriteString(selectedStyle.Render("  > " + addLabel))
	} else {
		b.WriteString(normalStyle.Render("    " + addLabel))
	}

	b.WriteString("\n\n")
	b.WriteString(hintStyle.Render("[↑/↓] navigate  [Enter] select  [D] delete"))

	box := boxStyle.Render(b.String())

	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
}
