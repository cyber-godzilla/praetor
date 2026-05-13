package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// paneLine is one entry in the output buffer. For normal lines,
// placeholder is nil and original carries the rendered segments. For
// suppressed lines, both fields are populated; the View renders the
// placeholder when the pane is collapsed and the original when expanded.
type paneLine struct {
	placeholder []types.StyledSegment // nil for normal lines
	original    []types.StyledSegment // always populated
}

// OutputPane is a scrollable text buffer for the All, Combat, and Social tabs.
type OutputPane struct {
	lines     []paneLine
	maxLines  int
	width     int
	height    int
	scrollPos int // 0 = bottom (auto-scroll)
	expanded  bool

	// Render cache: we keep previously rendered display rows so View()
	// only needs to render lines appended since the last call.
	// rowCounts tracks how many display rows each logical line produced,
	// allowing us to trim cachedRows in sync with lines instead of
	// invalidating the entire cache at scrollback capacity.
	cachedRows  []string // rendered display rows
	rowCounts   []int    // display rows per logical line (parallel to lines)
	cachedLines int      // number of logical lines already rendered
	cacheWidth  int      // width used for cached renders (invalidate on change)

	// Join cache: the joined viewport string returned by View(). Reused
	// when (start, end, height) match the prior call — i.e., no new
	// lines were appended, scrollPos didn't shift the visible window,
	// and the pane didn't resize. Bubbletea fires View() on every
	// event including cursor blink, so this saves a strings.Builder
	// pass per idle frame.
	joinedCache  string
	joinedStart  int
	joinedEnd    int
	joinedHeight int
}

// NewOutputPane creates a new OutputPane with the given scrollback limit.
func NewOutputPane(maxLines int) OutputPane {
	if maxLines <= 0 {
		maxLines = 1000
	}
	return OutputPane{
		maxLines: maxLines,
	}
}

// Append adds a normal line to the buffer.
func (o *OutputPane) Append(segments []types.StyledSegment) {
	o.appendLine(paneLine{original: segments})
}

// AppendSuppressed adds a line that has both a collapsed placeholder
// rendition and an expanded original rendition. The View renders one
// or the other depending on the pane's expand state.
func (o *OutputPane) AppendSuppressed(placeholder, original []types.StyledSegment) {
	o.appendLine(paneLine{placeholder: placeholder, original: original})
}

// appendLine adds one paneLine, trimming oldest entries when the
// scrollback exceeds maxLines (and trimming the render cache in sync).
func (o *OutputPane) appendLine(line paneLine) {
	o.lines = append(o.lines, line)
	if len(o.lines) > o.maxLines {
		excess := len(o.lines) - o.maxLines
		o.lines = o.lines[excess:]

		// Trim cachedRows in sync using per-line row counts.
		if len(o.rowCounts) >= excess {
			trimRows := 0
			for _, n := range o.rowCounts[:excess] {
				trimRows += n
			}
			o.rowCounts = o.rowCounts[excess:]
			o.cachedRows = o.cachedRows[trimRows:]
			o.cachedLines -= excess
			if o.cachedLines < 0 {
				o.cachedLines = 0
			}
		} else {
			// Row counts out of sync — full invalidation as fallback.
			o.cachedRows = nil
			o.rowCounts = nil
			o.cachedLines = 0
		}
		// Trimming shifts row indices; joined-string cache is stale.
		o.joinedCache = ""

		if o.scrollPos > 0 {
			o.scrollPos -= excess
			if o.scrollPos < 0 {
				o.scrollPos = 0
			}
		}
	}
}

// SetExpanded flips the global reveal mode for this pane. When true,
// View renders the original styled segments for every paneLine; when
// false, the placeholder is shown for paneLines that have one (normal
// lines always render their original segments).
func (o *OutputPane) SetExpanded(expanded bool) {
	if o.expanded == expanded {
		return
	}
	o.expanded = expanded
	o.cachedRows = nil
	o.rowCounts = nil
	o.cachedLines = 0
	o.joinedCache = ""
}

// ScrollUp scrolls up by n display rows.
func (o *OutputPane) ScrollUp(n int) {
	o.scrollPos += n
	// Cap is handled in View — we just let it go high and clamp later.
}

// ScrollDown scrolls down by n display rows. Scrolling past 0 snaps to bottom.
func (o *OutputPane) ScrollDown(n int) {
	o.scrollPos -= n
	if o.scrollPos < 0 {
		o.scrollPos = 0
	}
}

// SetSize updates the dimensions for the output pane.
func (o *OutputPane) SetSize(w, h int) {
	if w != o.width {
		// Width changed — word-wrap positions are different, invalidate cache.
		o.cachedRows = nil
		o.rowCounts = nil
		o.cachedLines = 0
		o.joinedCache = ""
	}
	o.width = w
	o.height = h
}

// View renders the visible portion of the output buffer with word wrapping.
func (o *OutputPane) View() string {
	if o.height <= 0 || o.width <= 0 {
		return ""
	}

	if len(o.lines) == 0 {
		return strings.Repeat("\n", o.height-1)
	}

	// Invalidate cache if width changed.
	if o.cacheWidth != o.width {
		o.cachedRows = nil
		o.rowCounts = nil
		o.cachedLines = 0
		o.cacheWidth = o.width
		o.joinedCache = ""
	}

	// Render only lines added since the last call.
	if o.cachedLines < len(o.lines) {
		for _, line := range o.lines[o.cachedLines:] {
			var segments []types.StyledSegment
			if line.placeholder != nil && !o.expanded {
				segments = line.placeholder
			} else {
				segments = line.original
			}
			rendered := renderSegments(segments, o.width)
			parts := strings.Split(rendered, "\n")
			o.cachedRows = append(o.cachedRows, parts...)
			o.rowCounts = append(o.rowCounts, len(parts))
		}
		o.cachedLines = len(o.lines)
		o.joinedCache = ""
	}

	totalRows := len(o.cachedRows)

	// Clamp scrollPos.
	maxScroll := totalRows - o.height
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollPos := o.scrollPos
	if scrollPos > maxScroll {
		scrollPos = maxScroll
	}

	// Calculate visible window from the bottom.
	end := totalRows - scrollPos
	start := end - o.height
	if start < 0 {
		start = 0
	}
	if end > totalRows {
		end = totalRows
	}

	// Reuse the joined string when the visible window is unchanged.
	if o.joinedCache != "" &&
		o.joinedStart == start &&
		o.joinedEnd == end &&
		o.joinedHeight == o.height {
		return o.joinedCache
	}

	visible := o.cachedRows[start:end]

	var b strings.Builder
	for i, row := range visible {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(row)
	}

	// Pad with empty lines if we have fewer rows than height.
	rendered := len(visible)
	for rendered < o.height {
		b.WriteByte('\n')
		rendered++
	}

	o.joinedCache = b.String()
	o.joinedStart = start
	o.joinedEnd = end
	o.joinedHeight = o.height
	return o.joinedCache
}

// renderSegments renders a slice of StyledSegments into a styled string,
// wrapping long lines at maxWidth by inserting newlines.
func renderSegments(segments []types.StyledSegment, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	// Check for HR segments and expand them to full-width horizontal lines.
	segments = expandHRSegments(segments, maxWidth)

	// Build plain text to compute wrap points, then re-apply styles.
	var plain strings.Builder
	for _, seg := range segments {
		plain.WriteString(seg.Text)
	}
	plainText := plain.String()

	// Compute wrap points: positions in the plain text where we insert newlines.
	// We prefer breaking at the last whitespace before the line limit.
	var wrapPoints []int
	col := 0
	lastSpace := -1   // byte index of the last space on the current line
	lastSpaceCol := 0 // column position of that space
	runes := []rune(plainText)
	for i, r := range runes {
		if r == '\n' {
			col = 0
			lastSpace = -1
			continue
		}
		if r == ' ' || r == '\t' {
			lastSpace = i
			lastSpaceCol = col
		}
		col++
		if col > maxWidth {
			if lastSpace > 0 && lastSpaceCol > 0 {
				// Wrap at the last whitespace: the newline replaces the space.
				wrapPoints = append(wrapPoints, lastSpace)
				// Recompute col: characters after the space are on the new line.
				col = col - lastSpaceCol - 1
				lastSpace = -1
			} else {
				// No whitespace found — hard break at current position.
				wrapPoints = append(wrapPoints, i)
				col = 1
				lastSpace = -1
			}
		}
	}

	// If no wrapping needed and no styling, fast path.
	if len(wrapPoints) == 0 && len(segments) == 1 && segments[0].Color == "" &&
		!segments[0].Bold && !segments[0].Italic && !segments[0].Underline {
		return plainText
	}

	// Render with styles and wrap insertions.
	// Walk through segments, tracking position in the plain text.
	var b strings.Builder
	plainPos := 0
	wrapIdx := 0

	for _, seg := range segments {
		style := lipgloss.NewStyle()
		if seg.Bold {
			style = style.Bold(true)
		}
		if seg.Italic {
			style = style.Italic(true)
		}
		if seg.Underline {
			style = style.Underline(true)
		}

		// Check for highlight style prefix.
		noStyle := false
		if strings.HasPrefix(seg.Color, "highlight:") {
			styleName := seg.Color[len("highlight:"):]
			if hs, ok := highlightStyles[styleName]; ok {
				style = hs.Bold(seg.Bold).Underline(seg.Underline)
			}
		} else if seg.Color != "" {
			style = style.Foreground(lipgloss.Color(seg.Color))
		} else {
			noStyle = !seg.Bold && !seg.Italic && !seg.Underline
		}

		// Process this segment's text, inserting wrap newlines as needed.
		var piece strings.Builder
		for _, r := range seg.Text {
			// Check if we need to insert a wrap at this position.
			if wrapIdx < len(wrapPoints) && plainPos == wrapPoints[wrapIdx] {
				// Flush current piece with style, then newline.
				if piece.Len() > 0 {
					if noStyle {
						b.WriteString(piece.String())
					} else {
						b.WriteString(style.Render(piece.String()))
					}
					piece.Reset()
				}
				b.WriteByte('\n')
				wrapIdx++

				// If wrapping at a space, skip the space so it doesn't
				// appear at the start of the next line.
				if r == ' ' || r == '\t' {
					plainPos++
					continue
				}
			}

			piece.WriteRune(r)

			if r != '\n' {
				plainPos++
			} else {
				plainPos++
			}
		}
		// Flush remaining piece.
		if piece.Len() > 0 {
			if noStyle {
				b.WriteString(piece.String())
			} else {
				b.WriteString(style.Render(piece.String()))
			}
		}
	}

	return b.String()
}

// expandHRSegments replaces IsHR segments with a full-width horizontal line.
// If the line contains text alongside an HR (e.g., a titlebar pattern), the
// text is centered between horizontal rules.
func expandHRSegments(segments []types.StyledSegment, maxWidth int) []types.StyledSegment {
	hasHR := false
	for _, seg := range segments {
		if seg.IsHR {
			hasHR = true
			break
		}
	}
	if !hasHR {
		return segments
	}

	// Collect non-HR text segments.
	var textSegs []types.StyledSegment
	totalTextLen := 0
	for _, seg := range segments {
		if seg.IsHR {
			continue
		}
		trimmed := strings.TrimSpace(seg.Text)
		if trimmed != "" {
			textSegs = append(textSegs, types.StyledSegment{
				Text:      trimmed,
				Bold:      seg.Bold,
				Italic:    seg.Italic,
				Underline: seg.Underline,
				Color:     seg.Color,
			})
			totalTextLen += len(trimmed)
		}
	}

	hrChar := "─"

	// Pure HR with no text: full-width line.
	if totalTextLen == 0 {
		return []types.StyledSegment{{
			Text:  strings.Repeat(hrChar, maxWidth),
			Color: "#888888",
		}}
	}

	// Titlebar: center text between horizontal rules.
	// Format: "──── Title Text ────"
	padding := 1 // space on each side of title
	sideLen := (maxWidth - totalTextLen - padding*2) / 2
	if sideLen < 2 {
		sideLen = 2
	}

	leftRule := strings.Repeat(hrChar, sideLen) + " "
	rightRule := " " + strings.Repeat(hrChar, sideLen)

	var result []types.StyledSegment
	result = append(result, types.StyledSegment{
		Text:  leftRule,
		Color: "#888888",
	})
	result = append(result, textSegs...)
	result = append(result, types.StyledSegment{
		Text:  rightRule,
		Color: "#888888",
	})
	return result
}
