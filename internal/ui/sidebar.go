package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/graphics"
	"github.com/cyber-godzilla/praetor/internal/kitty"
	"github.com/cyber-godzilla/praetor/internal/minimap"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Stable kitty image IDs for the sidebar's two graphics. Re-emitting
// with the same id atomically replaces the existing image in place
// (no visible flash). Sixel ignores these.
const (
	minimapImageID = 1
	compassImageID = 2
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
	graphicsMode  graphics.Mode

	// Graphics cache — avoid re-rendering the PNG every frame.
	graphicsDirty    bool
	cachedMinimapEsc string
	cachedCompassEsc string

	// emittedImages tracks whether the kitty images currently exist
	// in the terminal. ConsumeGraphics sets it true; HideGraphics
	// (sidebar collapse) and InvalidateGraphics (overlay's delete-all)
	// set it false. With kitty image IDs in use, re-emitting the same
	// image is an atomic replace — no flicker — so kitty path emits on
	// every game frame. Sixel uses dirtySinceEmit since its protocol
	// has no equivalent of in-place replacement.
	emittedImages            bool
	dirtySinceEmit           bool // sixel-only: emit only when underlying data changed
	cachedPlaceholder        string
	cachedCompassPlaceholder string
}

// NewSidebar creates a new Sidebar with the given minimap scale, height, and graphics mode.
func NewSidebar(minimapScale float64, minimapHeight int, mode graphics.Mode) Sidebar {
	mm := minimap.NewMinimap()
	mm.SetScale(minimapScale)
	return Sidebar{
		mode:           "disable",
		health:         100,
		fatigue:        100,
		satiation:      100,
		minimapHeight:  minimapHeight,
		minimap:        mm,
		graphicsMode:   mode,
		graphicsDirty:  true,
		dirtySinceEmit: true,
	}
}

// newSidebarPtr returns a pointer to a new Sidebar (for embedding in App).
func newSidebarPtr(minimapScale float64, minimapHeight int, mode graphics.Mode) *Sidebar {
	s := NewSidebar(minimapScale, minimapHeight, mode)
	return &s
}

// SetSize updates the sidebar dimensions.
func (s *Sidebar) SetSize(w, h int) {
	if s.width != w || s.height != h {
		s.graphicsDirty = true
		s.dirtySinceEmit = true
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
		s.graphicsDirty = true
		s.dirtySinceEmit = true
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
	s.graphicsDirty = true
	s.dirtySinceEmit = true
}

// MinimapHeight returns the minimap display height in terminal rows.
func (s Sidebar) MinimapHeight() int {
	return s.minimapHeight
}

// rebuildGraphicsCache re-renders minimap and compass images and caches the results.
func (s *Sidebar) rebuildGraphicsCache() {
	innerW := s.width - 2
	if innerW < 4 {
		innerW = 4
	}
	s.cachedPlaceholder, s.cachedMinimapEsc = s.minimap.Render(s.graphicsMode, minimapImageID)
	s.cachedCompassPlaceholder, s.cachedCompassEsc = compass.Render(s.graphicsMode, s.exits, innerW, compassImageID)
	s.graphicsDirty = false
}

// ConsumeGraphics returns the kitty/sixel escape sequences to inject
// into the current frame. The escapes are returned on every game
// frame, regardless of whether the underlying data changed:
//
//   - Kitty: each emit replaces the image at a fixed image id
//     atomically, so re-emitting is free of visible flicker and
//     self-heals if the terminal silently lost the image.
//   - Sixel: pixels live inline in the terminal's cell buffer. Any
//     surrounding text write overwrites them, so we must re-emit
//     every frame to keep the image visible. Sixel does flicker
//     briefly on each frame redraw — the alternative was "image
//     disappears between data updates," which is worse.
//
// The caller is responsible for positioning the escapes (cursor save +
// goto + restore) before injecting them into the rendered frame.
func (s *Sidebar) ConsumeGraphics() (minimap, compass string) {
	if s.graphicsDirty {
		s.rebuildGraphicsCache()
	}
	s.dirtySinceEmit = false
	s.emittedImages = true
	return s.cachedMinimapEsc, s.cachedCompassEsc
}

// HideGraphics returns escape sequences that surgically delete the
// sidebar's images from the terminal, without affecting other images.
// Used when the sidebar is collapsed (Alt+S) so the kitty image
// doesn't linger over the now-empty sidebar slot. Returns "" if no
// images are currently emitted, or if the graphics mode lacks an
// equivalent (sixel pixels are overwritten by normal text writes).
func (s *Sidebar) HideGraphics() string {
	if !s.emittedImages {
		return ""
	}
	s.emittedImages = false
	s.dirtySinceEmit = true // ensure we re-emit when sidebar comes back
	if s.graphicsMode != graphics.ModeKitty {
		return ""
	}
	return kitty.DeleteByID(minimapImageID) + kitty.DeleteByID(compassImageID)
}

// InvalidateGraphics tells the sidebar that an external action (e.g.
// an overlay view's delete-all escape) wiped the terminal images.
// Next ConsumeGraphics will re-emit unconditionally.
func (s *Sidebar) InvalidateGraphics() {
	s.emittedImages = false
	s.dirtySinceEmit = true
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
	if s.graphicsDirty {
		s.rebuildGraphicsCache()
	}
	if s.cachedPlaceholder != "" {
		sections = append(sections, s.cachedPlaceholder)
	}

	// Compass rose (rendered as Kitty graphic, placeholder for layout)
	compassPlaceholder := s.cachedCompassPlaceholder
	if compassPlaceholder == "" {
		compassPlaceholder = compass.View(innerWidth)
	}
	sections = append(sections, compassPlaceholder)

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
