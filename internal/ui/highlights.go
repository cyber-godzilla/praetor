package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/textutil"
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
			hm.editBuf = textutil.TrimLastRune(hm.editBuf)
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

type highlightSpan struct {
	start, end int
	style      string
}

// applyHighlights matches highlight patterns against the WHOLE line first, then
// splits the segment list at match edges. Matching per-segment (as before) meant
// a pattern spanning a colorword/style boundary — "gold ring", where colorwords
// puts "gold" in its own segment — never matched, silently breaking the
// feature's core loot use case.
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

	// Reassemble the full line; segment texts concatenate to it by construction.
	var full strings.Builder
	for _, seg := range segments {
		full.WriteString(seg.Text)
	}
	text := full.String()

	spans := findHighlightSpans(textutil.ToLowerASCII(text), active)
	if len(spans) == 0 {
		return segments
	}

	// Walk segments, splitting each at the span boundaries that fall inside it.
	var result []types.StyledSegment
	pos := 0     // byte offset of the current segment's start within text
	spanIdx := 0 // monotonic cursor into spans (cur only ever advances)
	for _, seg := range segments {
		segStart := pos
		segEnd := pos + len(seg.Text)
		pos = segEnd

		if seg.IsHR || seg.Text == "" {
			result = append(result, seg)
			continue
		}

		cur := segStart
		for cur < segEnd {
			sp := nextSpanOverlapping(spans, &spanIdx, cur, segEnd)
			if sp == nil {
				result = append(result, sliceStyled(seg, text[cur:segEnd]))
				break
			}
			if sp.start > cur {
				result = append(result, sliceStyled(seg, text[cur:sp.start]))
				cur = sp.start
			}
			end := sp.end
			if end > segEnd {
				end = segEnd // span continues into the next segment
			}
			result = append(result, types.StyledSegment{
				Text:      text[cur:end],
				Bold:      true,
				Italic:    seg.Italic,
				Underline: true,
				Color:     "highlight:" + sp.style,
			})
			cur = end
		}
	}
	return result
}

// findHighlightSpans returns non-overlapping match spans over the (length-
// preserving folded) full line. Precedence is config order: an earlier-
// configured pattern's match wins over a later pattern that would overlap it.
// The ASCII fold keeps byte offsets valid against the original text (a fold that
// changed byte length would slice out of range or tear a rune — see
// internal/textutil.ToLowerASCII).
func findHighlightSpans(lower string, highlights []config.HighlightConfig) []highlightSpan {
	var accepted []highlightSpan
	for _, h := range highlights {
		pattern := textutil.ToLowerASCII(h.Pattern)
		if pattern == "" {
			continue
		}
		offset := 0
		for {
			idx := strings.Index(lower[offset:], pattern)
			if idx < 0 {
				break
			}
			start := offset + idx
			end := start + len(pattern)
			offset = end
			if !overlapsAny(accepted, start, end) {
				accepted = append(accepted, highlightSpan{start, end, h.Style})
			}
		}
	}
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].start < accepted[j].start })
	return accepted
}

func overlapsAny(spans []highlightSpan, start, end int) bool {
	for _, s := range spans {
		if start < s.end && end > s.start {
			return true
		}
	}
	return false
}

// nextSpanOverlapping returns the first span at or after *from that overlaps
// [lo, hi), or nil if none do. It advances *from past spans that end at or
// before lo — since the walk's cursor is monotonic, those can never overlap
// again — making the whole walk O(segments + spans) instead of O(segments ×
// spans). Spans are sorted by start; a span whose start is >= hi (and all after
// it) can't overlap [lo, hi). It never advances past a span still in play (one
// straddling into the next segment has end > lo).
func nextSpanOverlapping(spans []highlightSpan, from *int, lo, hi int) *highlightSpan {
	for *from < len(spans) && spans[*from].end <= lo {
		*from++
	}
	if *from < len(spans) && spans[*from].start < hi {
		return &spans[*from]
	}
	return nil
}

// sliceStyled returns a copy of seg carrying s as its text and all of seg's
// original style fields.
func sliceStyled(seg types.StyledSegment, s string) types.StyledSegment {
	seg.Text = s
	return seg
}
