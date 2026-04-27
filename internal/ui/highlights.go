package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// Highlight style definitions: red, gold, green, blue.
var highlightStyles = map[string]lipgloss.Style{
	"red":   lipgloss.NewStyle().Background(lipgloss.Color("#cc0000")).Foreground(lipgloss.Color("#ffffff")),
	"gold":  lipgloss.NewStyle().Background(lipgloss.Color("#cc9900")).Foreground(lipgloss.Color("#000000")),
	"green": lipgloss.NewStyle().Background(lipgloss.Color("#00aa00")).Foreground(lipgloss.Color("#000000")),
	"blue":  lipgloss.NewStyle().Background(lipgloss.Color("#0055cc")).Foreground(lipgloss.Color("#ffffff")),
}

var styleNames = []string{"red", "gold", "green", "blue"}

// MenuHighlightsMsg signals the menu to open the highlights manager.
type MenuHighlightsMsg struct{}

// HighlightsCloseMsg is sent when the highlights manager is dismissed.
type HighlightsCloseMsg struct {
	Highlights []config.HighlightConfig
}

// HighlightsManager lets the user add, remove, toggle, and change highlight patterns.
type HighlightsManager struct {
	highlights []config.HighlightConfig
	cursor     int
	editing    bool   // true when typing a new pattern
	editBuf    string // buffer for new pattern input
	width      int
	height     int
}

func NewHighlightsManager(highlights []config.HighlightConfig) HighlightsManager {
	// Deep copy so edits don't affect the original until save.
	h := make([]config.HighlightConfig, len(highlights))
	copy(h, highlights)
	return HighlightsManager{
		highlights: h,
	}
}

func (hm *HighlightsManager) SetSize(w, h int) {
	hm.width = w
	hm.height = h
}

func (hm HighlightsManager) Update(msg tea.KeyMsg) (HighlightsManager, tea.Cmd) {
	if hm.editing {
		return hm.updateEditing(msg)
	}

	switch msg.Type {
	case tea.KeyEscape:
		return hm, func() tea.Msg {
			return HighlightsCloseMsg{Highlights: hm.highlights}
		}

	case tea.KeyUp:
		if hm.cursor > 0 {
			hm.cursor--
		}
		return hm, nil

	case tea.KeyDown:
		// +1 for the "Add new..." item at the bottom.
		max := len(hm.highlights)
		if hm.cursor < max {
			hm.cursor++
		}
		return hm, nil

	case tea.KeyEnter, tea.KeySpace:
		if msg.Type == tea.KeySpace {
			// Toggle active.
			if hm.cursor < len(hm.highlights) {
				hm.highlights[hm.cursor].Active = !hm.highlights[hm.cursor].Active
			}
			return hm, nil
		}
		if hm.cursor == len(hm.highlights) {
			// "Add new..." selected — enter editing mode.
			hm.editing = true
			hm.editBuf = ""
			return hm, nil
		}
		return hm, nil

	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case ' ':
				// Toggle active (fallback for terminals that send space as rune).
				if hm.cursor < len(hm.highlights) {
					hm.highlights[hm.cursor].Active = !hm.highlights[hm.cursor].Active
				}
			case 's':
				// Cycle style.
				if hm.cursor < len(hm.highlights) {
					cur := hm.highlights[hm.cursor].Style
					for i, name := range styleNames {
						if name == cur {
							hm.highlights[hm.cursor].Style = styleNames[(i+1)%len(styleNames)]
							break
						}
					}
				}
			case 'd':
				// Delete highlight.
				if hm.cursor < len(hm.highlights) {
					hm.highlights = append(hm.highlights[:hm.cursor], hm.highlights[hm.cursor+1:]...)
					if hm.cursor >= len(hm.highlights) && hm.cursor > 0 {
						hm.cursor--
					}
				}
			}
		}
		return hm, nil

	case tea.KeyDelete, tea.KeyBackspace:
		if hm.cursor < len(hm.highlights) {
			hm.highlights = append(hm.highlights[:hm.cursor], hm.highlights[hm.cursor+1:]...)
			if hm.cursor >= len(hm.highlights) && hm.cursor > 0 {
				hm.cursor--
			}
		}
		return hm, nil
	}

	return hm, nil
}

func (hm HighlightsManager) updateEditing(msg tea.KeyMsg) (HighlightsManager, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		hm.editing = false
		return hm, nil

	case tea.KeyEnter:
		pattern := strings.TrimSpace(hm.editBuf)
		if pattern != "" {
			hm.highlights = append(hm.highlights, config.HighlightConfig{
				Pattern: pattern,
				Style:   "gold",
				Active:  true,
			})
			hm.cursor = len(hm.highlights) - 1
		}
		hm.editing = false
		return hm, nil

	case tea.KeyBackspace:
		if len(hm.editBuf) > 0 {
			hm.editBuf = hm.editBuf[:len(hm.editBuf)-1]
		}
		return hm, nil

	case tea.KeyRunes:
		hm.editBuf += string(msg.Runes)
		return hm, nil

	case tea.KeySpace:
		hm.editBuf += " "
		return hm, nil
	}

	return hm, nil
}

func (hm HighlightsManager) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
	boxWidth := hm.width - 10
	if boxWidth < 40 {
		boxWidth = 40
	}
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorOrange).
		Padding(1, 3).
		Width(boxWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Highlights"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorDim).
		Render("[Space] toggle  [S] style  [D] delete  [Esc] save"))
	b.WriteString("\n\n")

	// Calculate viewport
	totalItems := len(hm.highlights) + 1 // +1 for "Add new" item
	maxVisible := hm.height - 12
	if maxVisible < 3 {
		maxVisible = 3
	}
	start := viewportWindow(totalItems, maxVisible, hm.cursor)
	end := start + maxVisible
	if end > totalItems {
		end = totalItems
	}

	for idx := start; idx < end; idx++ {
		if idx < len(hm.highlights) {
			i := idx
			h := hm.highlights[i]
			check := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render("○ ")
			if h.Active {
				check = lipgloss.NewStyle().Foreground(colorGreen).Render("● ")
			}
			style, ok := highlightStyles[h.Style]
			if !ok {
				style = highlightStyles["gold"]
			}
			preview := style.Render(" " + h.Style + " ")
			patternStyle := lipgloss.NewStyle().Foreground(colorDim)
			cursor := "    "
			if i == hm.cursor {
				patternStyle = lipgloss.NewStyle().Foreground(colorOrange).Bold(true)
				cursor = "  > "
			}
			b.WriteString(cursor + check + preview + " " + patternStyle.Render(h.Pattern))
			b.WriteByte('\n')
		} else {
			// "Add new" item
			if hm.cursor == len(hm.highlights) {
				if hm.editing {
					b.WriteString("  > " + lipgloss.NewStyle().Foreground(colorOrange).Render("+ Pattern: "+hm.editBuf+"█"))
				} else {
					b.WriteString("  > " + lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("+ Add new highlight..."))
				}
			} else {
				if hm.editing {
					b.WriteString("    " + lipgloss.NewStyle().Foreground(colorDim).Render("+ Pattern: "+hm.editBuf+"█"))
				} else {
					b.WriteString("    " + lipgloss.NewStyle().Foreground(colorDim).Render("+ Add new highlight..."))
				}
			}
			b.WriteByte('\n')
		}
	}

	return lipgloss.Place(hm.width, hm.height, lipgloss.Center, lipgloss.Center,
		boxStyle.Render(b.String()))
}

// applyHighlights splits styled segments where highlight patterns match,
// applying the highlight style to matched portions.
func applyHighlights(segments []types.StyledSegment, highlights []config.HighlightConfig) []types.StyledSegment {
	// Collect active highlights.
	var active []config.HighlightConfig
	for _, h := range highlights {
		if h.Active && h.Pattern != "" {
			active = append(active, h)
		}
	}
	if len(active) == 0 {
		return segments
	}

	var result []types.StyledSegment
	for _, seg := range segments {
		result = append(result, splitSegment(seg, active)...)
	}
	return result
}

// splitSegment splits a single segment wherever highlight patterns match.
func splitSegment(seg types.StyledSegment, highlights []config.HighlightConfig) []types.StyledSegment {
	if seg.IsHR || seg.Text == "" {
		return []types.StyledSegment{seg}
	}

	text := seg.Text
	lower := strings.ToLower(text)

	// Find all match ranges.
	type matchRange struct {
		start, end int
		style      string
	}
	var matches []matchRange

	for _, h := range highlights {
		pattern := strings.ToLower(h.Pattern)
		offset := 0
		for {
			idx := strings.Index(lower[offset:], pattern)
			if idx < 0 {
				break
			}
			start := offset + idx
			end := start + len(h.Pattern)
			matches = append(matches, matchRange{start, end, h.Style})
			offset = end
		}
	}

	if len(matches) == 0 {
		return []types.StyledSegment{seg}
	}

	// Sort matches by start position. Simple insertion sort (few matches expected).
	for i := 1; i < len(matches); i++ {
		j := i
		for j > 0 && matches[j].start < matches[j-1].start {
			matches[j], matches[j-1] = matches[j-1], matches[j]
			j--
		}
	}

	// Remove overlapping matches (keep first).
	var filtered []matchRange
	lastEnd := 0
	for _, m := range matches {
		if m.start >= lastEnd {
			filtered = append(filtered, m)
			lastEnd = m.end
		}
	}

	// Split text into segments.
	var result []types.StyledSegment
	pos := 0
	for _, m := range filtered {
		// Text before match — keeps original style.
		if m.start > pos {
			result = append(result, types.StyledSegment{
				Text:      text[pos:m.start],
				Bold:      seg.Bold,
				Italic:    seg.Italic,
				Underline: seg.Underline,
				Color:     seg.Color,
			})
		}
		// Matched text — uses highlight style.
		// We store the highlight style name in the Color field with a special prefix.
		result = append(result, types.StyledSegment{
			Text:      text[m.start:m.end],
			Bold:      true,
			Italic:    seg.Italic,
			Underline: true,
			Color:     "highlight:" + m.style,
		})
		pos = m.end
	}
	// Remaining text after last match.
	if pos < len(text) {
		result = append(result, types.StyledSegment{
			Text:      text[pos:],
			Bold:      seg.Bold,
			Italic:    seg.Italic,
			Underline: seg.Underline,
			Color:     seg.Color,
		})
	}

	return result
}
