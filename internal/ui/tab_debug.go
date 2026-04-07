package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/minimap"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// DebugPane displays raw SKOOT protocol data and a V2 minimap test render.
type DebugPane struct {
	width    int
	height   int
	scroll   int
	payloads map[int]string       // channel -> most recent raw payload
	rooms    []types.MinimapRoom  // parsed channel 6
	walls    []types.MinimapWall  // parsed channel 10
	exits    *types.Exits         // parsed channel 7
	lighting *types.LightingLevel // parsed channel 9
	mapV2    minimap.Minimap      // test renderer
}

const debugMapRows = 14 // rows for the V2 minimap in debug tab

// NewDebugPane creates a new DebugPane.
func NewDebugPane() DebugPane {
	v2 := minimap.NewMinimap()
	v2.SetSize(60, debugMapRows)
	return DebugPane{
		payloads: make(map[int]string),
		mapV2:    v2,
	}
}

// SetSize updates the pane dimensions.
func (d *DebugPane) SetSize(w, h int) {
	d.width = w
	d.height = h
	mapW := w
	if mapW > 80 {
		mapW = 80
	}
	d.mapV2.SetSize(mapW, debugMapRows)
}

// ScrollUp scrolls the debug pane up.
func (d *DebugPane) ScrollUp(lines int) {
	d.scroll -= lines
	if d.scroll < 0 {
		d.scroll = 0
	}
}

// ScrollDown scrolls the debug pane down.
func (d *DebugPane) ScrollDown(lines int) {
	d.scroll += lines
}

// UpdateSKOOT stores the latest raw payload and parsed data for a channel.
func (d *DebugPane) UpdateSKOOT(ev types.SKOOTUpdateEvent) {
	d.payloads[ev.Channel] = ev.RawPayload

	if len(ev.Rooms) > 0 {
		d.rooms = ev.Rooms
		d.mapV2.Update(ev.Rooms, nil)
	}
	if len(ev.Walls) > 0 {
		d.walls = ev.Walls
		d.mapV2.Update(nil, ev.Walls)
	}
	if ev.Exits != nil {
		d.exits = ev.Exits
	}
	if ev.Lighting != nil {
		d.lighting = ev.Lighting
	}
}

// View renders the debug data.
func (d DebugPane) View() string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	valueStyle := lipgloss.NewStyle().Foreground(colorGreen)
	rawStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#aaaaaa"))

	var lines []string

	// Channel 6: Minimap Rooms
	lines = append(lines, headerStyle.Render("SKOOT 6 — Minimap Rooms"))
	if raw, ok := d.payloads[6]; ok {
		lines = append(lines, labelStyle.Render("Raw: ")+rawStyle.Render(truncate(raw, d.width-6)))
		lines = append(lines, fmt.Sprintf("%s %s",
			labelStyle.Render("Rooms:"),
			valueStyle.Render(fmt.Sprintf("%d", len(d.rooms)))))
		for i, r := range d.rooms {
			color := "  "
			if r.Color == "#ff0000" {
				color = "* "
			}
			lines = append(lines, fmt.Sprintf("  %s%s",
				color,
				labelStyle.Render(fmt.Sprintf("[%d] x=%d y=%d size=%d color=%s bright=%.1f",
					i, r.X, r.Y, r.Size, r.Color, r.Brightness))))
		}
	} else {
		lines = append(lines, labelStyle.Render("  (no data)"))
	}

	lines = append(lines, "")

	// Channel 10: Walls
	lines = append(lines, headerStyle.Render("SKOOT 10 — Walls/Passages"))
	if raw, ok := d.payloads[10]; ok {
		lines = append(lines, labelStyle.Render("Raw: ")+rawStyle.Render(truncate(raw, d.width-6)))
		lines = append(lines, fmt.Sprintf("%s %s",
			labelStyle.Render("Walls:"),
			valueStyle.Render(fmt.Sprintf("%d", len(d.walls)))))
		for i, w := range d.walls {
			passable := "blocked"
			if w.Passable {
				passable = "PASSABLE"
			}
			lines = append(lines, fmt.Sprintf("  %s",
				labelStyle.Render(fmt.Sprintf("[%d] x=%d y=%d type=%s %s",
					i, w.X, w.Y, w.Type, passable))))
		}
	} else {
		lines = append(lines, labelStyle.Render("  (no data)"))
	}

	lines = append(lines, "")

	// Channel 7: Exits
	lines = append(lines, headerStyle.Render("SKOOT 7 — Exits"))
	if raw, ok := d.payloads[7]; ok {
		lines = append(lines, labelStyle.Render("Raw: ")+rawStyle.Render(truncate(raw, d.width-6)))
		if d.exits != nil {
			var active []string
			if d.exits.North {
				active = append(active, "N")
			}
			if d.exits.Northeast {
				active = append(active, "NE")
			}
			if d.exits.East {
				active = append(active, "E")
			}
			if d.exits.Southeast {
				active = append(active, "SE")
			}
			if d.exits.South {
				active = append(active, "S")
			}
			if d.exits.Southwest {
				active = append(active, "SW")
			}
			if d.exits.West {
				active = append(active, "W")
			}
			if d.exits.Northwest {
				active = append(active, "NW")
			}
			if d.exits.Up {
				active = append(active, "U")
			}
			if d.exits.Down {
				active = append(active, "D")
			}
			lines = append(lines, fmt.Sprintf("  %s %s",
				labelStyle.Render("Active:"),
				valueStyle.Render(strings.Join(active, " "))))
		}
	} else {
		lines = append(lines, labelStyle.Render("  (no data)"))
	}

	lines = append(lines, "")

	// Channel 9: Lighting
	lines = append(lines, headerStyle.Render("SKOOT 9 — Lighting"))
	if raw, ok := d.payloads[9]; ok {
		lines = append(lines, labelStyle.Render("Raw: ")+rawStyle.Render(raw))
	} else {
		lines = append(lines, labelStyle.Render("  (no data)"))
	}

	lines = append(lines, "")

	// Channel 8: Status
	lines = append(lines, headerStyle.Render("SKOOT 8 — Status"))
	if raw, ok := d.payloads[8]; ok {
		lines = append(lines, labelStyle.Render("Raw: ")+rawStyle.Render(raw))
	} else {
		lines = append(lines, labelStyle.Render("  (no data)"))
	}

	// Apply scroll
	if d.scroll >= len(lines) {
		d.scroll = len(lines) - 1
	}
	if d.scroll < 0 {
		d.scroll = 0
	}
	start := d.scroll
	end := start + d.height
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}
	visible := lines[start:end]

	return strings.Join(visible, "\n")
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
