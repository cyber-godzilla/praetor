package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/minimap"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Sidebar displays the minimap, compass rose, vitals bars, and mode info.
type Sidebar struct {
	width         int
	height        int
	minimapHeight int
	compact       bool // only show minimap + compass (no lighting/vitals)
	exits         types.Exits
	lighting      types.LightingLevel
	lightingRaw   int
	health        int
	fatigue       int
	encumbrance   int
	satiation     int
	mode          string
	displayState  []types.StateDisplayItem
	mapURL        string
	minimap       minimap.Minimap

	// Kitty graphics cache — avoid re-rendering every frame.
	kittyDirty        bool
	cachedMinimapEsc  string
	cachedCompassEsc  string
	cachedPlaceholder string
}

// NewSidebar creates a new Sidebar with the given minimap scale and height.
func NewSidebar(minimapScale float64, minimapHeight int) Sidebar {
	mm := minimap.NewMinimap()
	mm.SetScale(minimapScale)
	return Sidebar{
		mode:          "disable",
		health:        100,
		fatigue:       100,
		satiation:     100,
		minimapHeight: minimapHeight,
		minimap:       mm,
		kittyDirty:    true,
	}
}

// newSidebarPtr returns a pointer to a new Sidebar (for embedding in App).
func newSidebarPtr(minimapScale float64, minimapHeight int) *Sidebar {
	s := NewSidebar(minimapScale, minimapHeight)
	return &s
}

// SetSize updates the sidebar dimensions.
func (s *Sidebar) SetSize(w, h int) {
	if s.width != w || s.height != h {
		s.kittyDirty = true
	}
	s.width = w
	s.height = h
	innerW := w - 2
	if innerW < 4 {
		innerW = 4
	}
	s.minimap.SetSize(innerW, s.minimapHeight)
}

// SetCompact sets whether the sidebar should only show minimap and compass.
func (s *Sidebar) SetCompact(compact bool) {
	s.compact = compact
}

// UpdateExits updates the available exits for the compass rose.
func (s *Sidebar) UpdateExits(exits types.Exits) {
	if s.exits != exits {
		s.kittyDirty = true
	}
	s.exits = exits
}

// UpdateLighting updates the lighting level and raw value.
func (s *Sidebar) UpdateLighting(l types.LightingLevel, raw int) {
	s.lighting = l
	s.lightingRaw = raw
}

// UpdateVitals updates the health, fatigue, encumbrance, and satiation values.
func (s *Sidebar) UpdateVitals(health, fatigue, encumbrance, satiation *int) {
	if health != nil {
		s.health = *health
	}
	if fatigue != nil {
		s.fatigue = *fatigue
	}
	if encumbrance != nil {
		s.encumbrance = *encumbrance
	}
	if satiation != nil {
		s.satiation = *satiation
	}
}

// UpdateMode updates the current mode display.
func (s *Sidebar) UpdateMode(mode string) {
	s.mode = mode
}

// UpdateDisplayState updates the mode-declared state items for sidebar display.
func (s *Sidebar) UpdateDisplayState(items []types.StateDisplayItem) {
	s.displayState = items
}

// UpdateMapURL updates the minimap URL.
func (s *Sidebar) UpdateMapURL(url string) {
	s.mapURL = url
}

// UpdateMinimap updates the minimap room and wall data.
func (s *Sidebar) UpdateMinimap(rooms []types.MinimapRoom, walls []types.MinimapWall) {
	s.minimap.Update(rooms, walls)
	s.kittyDirty = true
}

// MinimapHeight returns the minimap display height in terminal rows.
func (s Sidebar) MinimapHeight() int {
	return s.minimapHeight
}

// rebuildKittyCache re-renders minimap and compass images and caches the results.
func (s *Sidebar) rebuildKittyCache() {
	innerW := s.width - 2
	if innerW < 4 {
		innerW = 4
	}
	s.cachedPlaceholder, s.cachedMinimapEsc = s.minimap.Render(graphics.ModeKitty)
	_, s.cachedCompassEsc = compass.Render(graphics.ModeKitty, s.exits, innerW)
	s.kittyDirty = false
}

// KittyEscapes returns Kitty graphics escape sequences for minimap and compass.
// Must be injected into the final output OUTSIDE of Lipgloss rendering.
// Returns (minimapEscape, compassEscape).
func (s *Sidebar) KittyEscapes() (string, string) {
	if s.kittyDirty {
		s.rebuildKittyCache()
	}
	return s.cachedMinimapEsc, s.cachedCompassEsc
}

// View renders the sidebar content.
func (s *Sidebar) View() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	innerWidth := s.width - 2 // Account for border
	if innerWidth < 1 {
		innerWidth = 1
	}

	var sections []string

	// Minimap (placeholder for Kitty image)
	if s.kittyDirty {
		s.rebuildKittyCache()
	}
	if s.cachedPlaceholder != "" {
		sections = append(sections, s.cachedPlaceholder)
	}

	// Compass rose (rendered as Kitty graphic, placeholder for layout)
	sections = append(sections, compass.View(innerWidth))

	if !s.compact {
		// Lighting
		sections = append(sections, s.renderLighting())

		// Vitals bars
		sections = append(sections, s.renderBar("HP", s.health, innerWidth))
		sections = append(sections, s.renderBar("FT", s.fatigue, innerWidth))
		sections = append(sections, s.renderBar("EN", s.encumbrance, innerWidth))
		sections = append(sections, s.renderBar("SA", s.satiation, innerWidth))
	}

	content := strings.Join(sections, "\n")

	return sidebarStyle.Width(s.width).Height(s.height).Render(content)
}

// renderLighting returns a styled lighting indicator.
func (s Sidebar) renderLighting() string {
	var symbol, label string
	var color lipgloss.Color

	switch s.lighting {
	case types.LightBlindinglyBright:
		symbol = "☀"
		label = "Blindingly Bright"
		color = lipgloss.Color("#ffffff")
	case types.LightVeryBright:
		symbol = "☀"
		label = "Very Brightly Lit"
		color = lipgloss.Color("#ffee66")
	case types.LightBright:
		symbol = "☀"
		label = "Brightly Lit"
		color = lipgloss.Color("#ffcc00")
	case types.LightFairlyLit:
		symbol = "◐"
		label = "Fairly Well-Lit"
		color = lipgloss.Color("#aa8800")
	case types.LightSomewhatDark:
		symbol = "◐"
		label = "Somewhat Dark"
		color = lipgloss.Color("#887744")
	case types.LightVeryDark:
		symbol = "☽"
		label = "Very Dark"
		color = lipgloss.Color("#6666aa")
	case types.LightExtremelyDark:
		symbol = "☽"
		label = "Extremely Dark"
		color = lipgloss.Color("#555588")
	case types.LightPitchBlack:
		symbol = "●"
		label = "Pitch Black"
		color = lipgloss.Color("#444444")
	default:
		symbol = "◐"
		label = "Fairly Well-Lit"
		color = lipgloss.Color("#aa8800")
	}

	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf(" %s %s (%d)", symbol, label, s.lightingRaw))
}

// renderBar renders a horizontal bar with label.
func (s Sidebar) renderBar(label string, value, maxWidth int) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	// Choose color based on thresholds
	var barColor lipgloss.Color
	switch {
	case value > 50:
		barColor = colorGreen
	case value > 25:
		barColor = colorOrange
	default:
		barColor = colorRed
	}

	// Label takes 4 chars " HP ", bar gets the rest
	barWidth := maxWidth - 5
	if barWidth < 1 {
		barWidth = 1
	}

	filled := value * barWidth / 100
	empty := barWidth - filled

	labelStr := lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf(" %s ", label))
	filledStr := lipgloss.NewStyle().Background(barColor).Render(strings.Repeat(" ", filled))
	emptyStr := lipgloss.NewStyle().Background(colorBarEmpty).Render(strings.Repeat(" ", empty))

	return labelStr + filledStr + emptyStr
}
