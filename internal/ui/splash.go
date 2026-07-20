package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// splashArt is the PRAETOR ASCII art. Version is injected at render time.
// Edit this string to tweak the splash screen appearance.
var splashArt = `  _._._                                                              _._._
 )_   _(                                                            )_   _(
   |_|                                                                |_|
   |║|  ██████╗ ██████╗  █████╗ ███████╗████████╗ ██████╗ ██████╗     |║|
   |║|  ██╔══██╗██╔══██╗██╔══██╗██╔════╝╚══██╔══╝██╔═══██╗██╔══██╗    |║|
   |║|  ██████╔╝██████╔╝███████║█████╗     ██║   ██║   ██║██████╔╝    |║|
   |║|  ██╔═══╝ ██╔══██╗██╔══██║██╔══╝     ██║   ██║   ██║██╔══██╗    |║|
   |║|  ██║     ██║  ██║██║  ██║███████╗   ██║   ╚██████╔╝██║  ██║    |║|
   |║|  ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝    |║|
   |║|                           ═══ ✦ ═══                            |║|
   |║|%s|║|
  _|_|_                                                              _|_|_
 |_____|                                                            |_____|`

// splashBoxInterior is the column width between the |║| borders of splashArt;
// the version line's %s is centered within it.
const splashBoxInterior = 64

// centerVersion centers version within a width-column field, padding with
// spaces on both sides so the splash box border stays aligned for any version
// length. Longer strings are clipped (the version was previously hard-truncated
// to 6 chars, dropping a digit at two-digit patch numbers like v0.2.10).
func centerVersion(version string, width int) string {
	if len(version) > width {
		version = version[:width]
	}
	total := width - len(version)
	left := total / 2
	return strings.Repeat(" ", left) + version + strings.Repeat(" ", total-left)
}

type splashTickMsg struct{}

// Splash is the startup splash screen showing PRAETOR branding.
type Splash struct {
	version  string
	showHint bool
	width    int
	height   int
}

// NewSplash creates a new splash screen with the given version string.
func NewSplash(version string) Splash {
	return Splash{version: version}
}

// SetSize updates the splash screen dimensions.
func (s *Splash) SetSize(w, h int) {
	s.width = w
	s.height = h
}

// Init starts the 1-second timer for showing the hint text.
func (s Splash) Init() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return splashTickMsg{}
	})
}

// Update handles messages for the splash screen.
func (s Splash) Update(msg tea.Msg) (Splash, tea.Cmd) {
	switch msg.(type) {
	case splashTickMsg:
		s.showHint = true
	}
	return s, nil
}

// View renders the splash screen centered in the terminal.
func (s Splash) View() string {
	artStyle := lipgloss.NewStyle().Foreground(colorOrange)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	// Center the version within the box interior so the border stays aligned for
	// any version length.
	art := fmt.Sprintf(splashArt, centerVersion(s.version, splashBoxInterior))

	var b strings.Builder
	b.WriteString(artStyle.Render(art))
	b.WriteString("\n\n")

	if s.showHint {
		b.WriteString(dimStyle.Render("Press any key to continue"))
	}

	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, b.String())
}
