package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/compass"
	"github.com/cyber-godzilla/praetor/internal/graphics"
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

// kittyDeleteAllSidebar is the escape sent to wipe sidebar/topbar
// kitty graphics. We use d=A (delete all) rather than d=I (delete by
// id) because some kitty-compatible terminals (notably foot's recent
// builds and certain wezterm versions) don't honor d=I,i=N reliably.
// Since the sidebar is currently the only thing in this app that
// emits kitty graphics, "delete all" is equivalent.
const kittyDeleteAllSidebar = "\033_Ga=d,d=A,q=2;\033\\"

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
	dirtySinceEmit           bool   // sixel-only: emit only when underlying data changed
	lastEmitAnchor           string // "sidebar" or "topbar" — used to detect mode-change transitions
	cachedPlaceholder        string
	cachedCompassPlaceholder string

	// Text-view cache — Bubbletea fires View() on every event (including
	// cursor blink), but the sidebar's text content only changes on a
	// handful of state mutations. We memoize the rendered string and
	// invalidate via viewDirty in the setters that affect it.
	viewDirty      bool
	cachedViewText string
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
		viewDirty:      true,
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
		s.viewDirty = true
		// On a dimension change, kitty's existing placements stay at
		// their old absolute cell positions — and our next emit at
		// the new positions creates new placements alongside them
		// (see kitty graphics protocol; a=T creates new placements
		// rather than moving them). Wipe them now so the next View
		// paints fresh, then mark the sidebar as needing a re-emit.
		if s.emittedImages && s.graphicsMode == graphics.ModeKitty {
			_, _ = os.Stdout.Write([]byte(kittyDeleteAllSidebar))
			s.emittedImages = false
			s.lastEmitAnchor = ""
		}
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
	if s.compact != compact {
		s.viewDirty = true
	}
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
	if s.lighting != l || s.lightingRaw != raw {
		s.viewDirty = true
	}
	s.lighting = l
	s.lightingRaw = raw
}

// UpdateVitals updates the health, fatigue, encumbrance, and satiation values.
func (s *Sidebar) UpdateVitals(health, fatigue, encumbrance, satiation *int) {
	if health != nil && *health != s.health {
		s.health = *health
		s.viewDirty = true
	}
	if fatigue != nil && *fatigue != s.fatigue {
		s.fatigue = *fatigue
		s.viewDirty = true
	}
	if encumbrance != nil && *encumbrance != s.encumbrance {
		s.encumbrance = *encumbrance
		s.viewDirty = true
	}
	if satiation != nil && *satiation != s.satiation {
		s.satiation = *satiation
		s.viewDirty = true
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
// Placeholders feed into the text View, so this also invalidates the view-text cache.
func (s *Sidebar) rebuildGraphicsCache() {
	innerW := s.width - 2
	if innerW < 4 {
		innerW = 4
	}
	s.cachedPlaceholder, s.cachedMinimapEsc = s.minimap.Render(s.graphicsMode, minimapImageID)
	s.cachedCompassPlaceholder, s.cachedCompassEsc = compass.Render(s.graphicsMode, s.exits, innerW, compassImageID)
	s.graphicsDirty = false
	s.viewDirty = true
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
// anchor identifies where the caller plans to place the images this
// frame (e.g. "sidebar" or "topbar"). When the anchor changes from
// the previous emission, transition contains kitty delete-by-id
// escapes for the previous placements so they don't linger at the old
// position; the caller must inject transition before the new escapes.
//
// The caller is responsible for positioning the escapes (cursor save +
// goto + restore) before injecting them into the rendered frame.
func (s *Sidebar) ConsumeGraphics(anchor string) (transition, minimap, compass string) {
	if s.graphicsDirty {
		s.rebuildGraphicsCache()
	}
	if s.emittedImages && s.lastEmitAnchor != "" && s.lastEmitAnchor != anchor {
		if s.graphicsMode == graphics.ModeKitty {
			transition = kittyDeleteAllSidebar
		}
	}
	s.lastEmitAnchor = anchor
	s.dirtySinceEmit = false
	s.emittedImages = true
	return transition, s.cachedMinimapEsc, s.cachedCompassEsc
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
	s.lastEmitAnchor = ""
	if s.graphicsMode != graphics.ModeKitty {
		return ""
	}
	return kittyDeleteAllSidebar
}

// InvalidateGraphics tells the sidebar that an external action (e.g.
// an overlay view's delete-all escape) wiped the terminal images.
// Next ConsumeGraphics will re-emit unconditionally.
func (s *Sidebar) InvalidateGraphics() {
	s.emittedImages = false
	s.dirtySinceEmit = true
	s.lastEmitAnchor = ""
}

// TopbarView renders the same data as View, but laid out horizontally
// across the top of the screen rather than vertically down a column.
// Three tiles, top-aligned: minimap (innerW × minimapHeight), compass
// (innerW × compass.Rows, padded with empty rows to minimapHeight tall),
// and a right panel with lighting + the four vital bars stretched to
// fill the remaining width. Returns a string of exactly minimapHeight
// rows separated by '\n'. The caller is responsible for placing it
// above the main content area.
func (s *Sidebar) TopbarView(totalWidth int) string {
	if s.graphicsDirty {
		s.rebuildGraphicsCache()
	}

	innerW := s.width - 2
	if innerW < 4 {
		innerW = 4
	}
	rows := s.minimapHeight
	if rows < 1 {
		rows = 1
	}

	pad := func(line string, w int) string {
		need := w - lipgloss.Width(line)
		if need <= 0 {
			return line
		}
		return line + strings.Repeat(" ", need)
	}
	emptyRow := strings.Repeat(" ", innerW)
	padTile := func(text string, w int) []string {
		split := strings.Split(text, "\n")
		out := make([]string, rows)
		for i := 0; i < rows; i++ {
			if i < len(split) {
				out[i] = pad(split[i], w)
			} else {
				out[i] = strings.Repeat(" ", w)
			}
		}
		return out
	}

	minimapLines := padTile(s.cachedPlaceholder, innerW)
	if minimapLines[0] == "" || lipgloss.Width(minimapLines[0]) == 0 {
		// Fallback if cache wasn't built (e.g., no rooms yet) — fill rows with whitespace.
		for i := range minimapLines {
			minimapLines[i] = emptyRow
		}
	}

	compassPlaceholder := s.cachedCompassPlaceholder
	if compassPlaceholder == "" {
		compassPlaceholder = compass.View(innerW)
	}
	compassLines := padTile(compassPlaceholder, innerW)

	// Right panel width: cap at innerW so the bars match their sidebar-
	// mode width rather than stretching to fill the remaining horizontal
	// space (which on a wide terminal would produce hugely elongated
	// bars). Cap fits within the available room minus the two tiles
	// and their gaps.
	available := totalWidth - innerW*2 - 2
	rightW := innerW
	if rightW > available {
		rightW = available
	}
	if rightW < 10 {
		rightW = 10
	}
	rightLines := make([]string, 0, rows)
	rightLines = append(rightLines, pad(s.renderLighting(), rightW))
	rightLines = append(rightLines, pad(s.renderBar("HP", s.health, rightW), rightW))
	rightLines = append(rightLines, pad(s.renderBar("FT", s.fatigue, rightW), rightW))
	rightLines = append(rightLines, pad(s.renderBar("EN", s.encumbrance, rightW), rightW))
	rightLines = append(rightLines, pad(s.renderBar("SA", s.satiation, rightW), rightW))
	for len(rightLines) < rows {
		rightLines = append(rightLines, strings.Repeat(" ", rightW))
	}
	rightLines = rightLines[:rows]

	gap := " "
	var lines []string
	for i := 0; i < rows; i++ {
		lines = append(lines, minimapLines[i]+gap+compassLines[i]+gap+rightLines[i])
	}
	return strings.Join(lines, "\n")
}

// View renders the sidebar content.
func (s *Sidebar) View() string {
	if s.width <= 0 || s.height <= 0 {
		return ""
	}

	if s.graphicsDirty {
		s.rebuildGraphicsCache()
	}
	if !s.viewDirty && s.cachedViewText != "" {
		return s.cachedViewText
	}

	innerWidth := s.width - 2 // Account for border
	if innerWidth < 1 {
		innerWidth = 1
	}

	var sections []string

	// Minimap (placeholder for Kitty image)
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

	s.cachedViewText = sidebarStyle.Width(s.width).Height(s.height).Render(content)
	s.viewDirty = false
	return s.cachedViewText
}

// renderLighting returns a styled lighting indicator.
func (s Sidebar) renderLighting() string {
	var symbol, label string
	var style lipgloss.Style

	switch s.lighting {
	case types.LightBlindinglyBright:
		symbol, label, style = "☀", "Blindingly Bright", lightingStyleBlinding
	case types.LightVeryBright:
		symbol, label, style = "☀", "Very Brightly Lit", lightingStyleVeryBright
	case types.LightBright:
		symbol, label, style = "☀", "Brightly Lit", lightingStyleBright
	case types.LightFairlyLit:
		symbol, label, style = "◐", "Fairly Well-Lit", lightingStyleFairlyLit
	case types.LightSomewhatDark:
		symbol, label, style = "◐", "Somewhat Dark", lightingStyleSomewhatDark
	case types.LightVeryDark:
		symbol, label, style = "☽", "Very Dark", lightingStyleVeryDark
	case types.LightExtremelyDark:
		symbol, label, style = "☽", "Extremely Dark", lightingStyleExtremeDark
	case types.LightPitchBlack:
		symbol, label, style = "●", "Pitch Black", lightingStylePitchBlack
	default:
		symbol, label, style = "◐", "Fairly Well-Lit", lightingStyleFairlyLit
	}

	return style.Render(fmt.Sprintf(" %s %s (%d)", symbol, label, s.lightingRaw))
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
	var fillStyle lipgloss.Style
	switch {
	case value > 50:
		fillStyle = barFillGreenStyle
	case value > 25:
		fillStyle = barFillOrangeStyle
	default:
		fillStyle = barFillRedStyle
	}

	// Label takes 4 chars " HP ", bar gets the rest
	barWidth := maxWidth - 5
	if barWidth < 1 {
		barWidth = 1
	}

	filled := value * barWidth / 100
	empty := barWidth - filled

	labelStr := barLabelStyle.Render(fmt.Sprintf(" %s ", label))
	filledStr := fillStyle.Render(strings.Repeat(" ", filled))
	emptyStr := barEmptyStyle.Render(strings.Repeat(" ", empty))

	return labelStr + filledStr + emptyStr
}
