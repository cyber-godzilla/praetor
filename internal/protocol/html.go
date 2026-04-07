package protocol

import (
	"html"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// HTMLResult holds the result of parsing an ALICECOMPAT HTML line.
type HTMLResult struct {
	Text         string                // Plain text with all tags stripped
	Segments     []types.StyledSegment // Styled segments preserving formatting info
	ClearPage    bool                  // True if <xch_page clear="text"> was encountered
	SectionBreak bool                  // True if </pre> was encountered (section boundary)
	HasHR        bool                  // True if <hr> was encountered
	IndentLevel  int                   // Current <ul> nesting depth after parsing this line
}

// styleState tracks the current style stack during HTML parsing.
type styleState struct {
	bold      bool
	italic    bool
	underline bool
	color     string
}

// ParseHTML parses ALICECOMPAT HTML into plain text and styled segments.
// Recognized tags:
//   - <b>        → Bold
//   - <i>        → Italic
//   - <font color="#hex"> → Color
//   - <xch_cmd>  → Underline
//   - <xch_page clear="text"> → ClearPage=true
//   - Other tags  → stripped, content preserved
//
// HTML entities (&lt; &gt; &amp; &quot;) are decoded.
// ParseHTML parses a single line of ALICECOMPAT HTML. The indentLevel parameter
// carries the current <ul> nesting depth from previous lines, since <ul> tags
// can span multiple protocol lines. The resulting IndentLevel should be passed
// to the next call.
func ParseHTML(input string) HTMLResult {
	return ParseHTMLWithIndent(input, 0)
}

// ParseHTMLWithIndent parses HTML with an initial indent level from prior lines.
func ParseHTMLWithIndent(input string, startIndent int) HTMLResult {
	if input == "" {
		return HTMLResult{IndentLevel: startIndent}
	}

	var result HTMLResult
	var textBuf strings.Builder
	var segments []types.StyledSegment

	// Style stack: each entry corresponds to a pushed tag.
	// We keep a running "current style" derived from the stack.
	type stackEntry struct {
		tag       string
		bold      bool
		italic    bool
		underline bool
		color     string
	}
	var stack []stackEntry
	cur := styleState{}
	indentLevel := startIndent // tracks <ul> nesting depth, carried from prior lines

	// segBuf accumulates text for the current styled segment.
	var segBuf strings.Builder

	flushSeg := func() {
		if segBuf.Len() > 0 {
			segments = append(segments, types.StyledSegment{
				Text:      segBuf.String(),
				Bold:      cur.bold,
				Italic:    cur.italic,
				Underline: cur.underline,
				Color:     cur.color,
			})
			segBuf.Reset()
		}
	}

	recalcStyle := func() {
		cur = styleState{}
		for _, e := range stack {
			if e.bold {
				cur.bold = true
			}
			if e.italic {
				cur.italic = true
			}
			if e.underline {
				cur.underline = true
			}
			if e.color != "" {
				cur.color = e.color
			}
		}
	}

	i := 0
	for i < len(input) {
		if input[i] == '<' {
			// Find end of tag.
			end := strings.IndexByte(input[i:], '>')
			if end < 0 {
				// Malformed: treat rest as text.
				decoded := html.UnescapeString(input[i:])
				textBuf.WriteString(decoded)
				segBuf.WriteString(decoded)
				break
			}
			tagContent := input[i+1 : i+end]
			i = i + end + 1

			// Self-closing tags like <br/>
			if strings.HasSuffix(tagContent, "/") {
				// Nothing to push/pop, just skip.
				continue
			}

			// Handle <hr> as a horizontal rule segment.
			if strings.EqualFold(strings.TrimSpace(tagContent), "hr") {
				flushSeg()
				segments = append(segments, types.StyledSegment{
					IsHR: true,
				})
				result.HasHR = true
				continue
			}

			// Closing tag?
			if strings.HasPrefix(tagContent, "/") {
				closeName := strings.TrimPrefix(tagContent, "/")
				closeName = strings.ToLower(strings.TrimSpace(closeName))
				if closeName == "pre" {
					result.SectionBreak = true
				}
				if closeName == "ul" {
					if indentLevel > 0 {
						indentLevel--
					}
					continue
				}
				// Pop matching stack entry.
				for j := len(stack) - 1; j >= 0; j-- {
					if stack[j].tag == closeName {
						flushSeg()
						stack = append(stack[:j], stack[j+1:]...)
						recalcStyle()
						break
					}
				}
				continue
			}

			// Opening tag: parse tag name and attributes.
			tagName, attrs := parseTag(tagContent)
			tagName = strings.ToLower(tagName)

			// Handle list tags specially — they affect indentation, not styling.
			if tagName == "ul" {
				indentLevel++
				continue
			}
			if tagName == "li" {
				// Inject indentation as text before the list item content.
				flushSeg()
				indent := strings.Repeat("    ", indentLevel)
				textBuf.WriteString(indent)
				segBuf.WriteString(indent)
				continue
			}

			entry := stackEntry{tag: tagName}
			switch tagName {
			case "b":
				entry.bold = true
			case "i":
				entry.italic = true
			case "xch_cmd":
				entry.underline = true
			case "font":
				if c, ok := attrs["color"]; ok {
					resolved := resolveColor(c)
					if resolved != "" {
						// Brighten dark hex colors for visibility on dark terminals.
						// Named colors are already curated, but hex values from
						// the game may be too dark (#00008B, #4B0082, etc.).
						brightened := brightenIfDark(resolved)
						if brightened != "" {
							entry.color = brightened
						}
					}
				}
				if s, ok := attrs["size"]; ok && strings.HasPrefix(s, "+") {
					entry.bold = true
					if entry.color == "" {
						entry.color = "#e8a838" // orange for title text
					}
				}
			case "xch_page":
				if c, ok := attrs["clear"]; ok && c == "text" {
					result.ClearPage = true
				}
			}

			flushSeg()
			stack = append(stack, entry)
			recalcStyle()
		} else {
			// Find the next tag or end of input.
			next := strings.IndexByte(input[i:], '<')
			var chunk string
			if next < 0 {
				chunk = input[i:]
				i = len(input)
			} else {
				chunk = input[i : i+next]
				i = i + next
			}
			decoded := html.UnescapeString(chunk)
			textBuf.WriteString(decoded)
			segBuf.WriteString(decoded)
		}
	}

	flushSeg()
	result.Text = textBuf.String()
	result.Segments = segments
	result.IndentLevel = indentLevel
	return result
}

// parseTag splits raw tag content (without < >) into the tag name and
// a map of attribute key=value pairs.
func parseTag(raw string) (string, map[string]string) {
	raw = strings.TrimSpace(raw)
	attrs := make(map[string]string)

	// Split on first space to get tag name.
	spaceIdx := strings.IndexByte(raw, ' ')
	if spaceIdx < 0 {
		return raw, attrs
	}
	tagName := raw[:spaceIdx]
	rest := raw[spaceIdx+1:]

	// Simple attribute parser: key="value" or key='value'
	for len(rest) > 0 {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		eqIdx := strings.IndexByte(rest, '=')
		if eqIdx < 0 {
			break
		}
		key := strings.TrimSpace(rest[:eqIdx])
		rest = rest[eqIdx+1:]
		rest = strings.TrimSpace(rest)
		if len(rest) == 0 {
			break
		}
		quote := rest[0]
		if quote != '"' && quote != '\'' {
			// Unquoted value: take until space.
			spIdx := strings.IndexByte(rest, ' ')
			if spIdx < 0 {
				attrs[key] = rest
				break
			}
			attrs[key] = rest[:spIdx]
			rest = rest[spIdx+1:]
			continue
		}
		rest = rest[1:]
		endQuote := strings.IndexByte(rest, quote)
		if endQuote < 0 {
			attrs[key] = rest
			break
		}
		attrs[key] = rest[:endQuote]
		rest = rest[endQuote+1:]
	}

	return tagName, attrs
}

// namedColors maps CSS/game color names to hex values.
// Includes all colors from TEC's color selection dialog.
var namedColors = map[string]string{
	"aqua":             "#00ffff",
	"averagepurple":    "#9955bb",
	"averagevioletred": "#cc3377",
	"black":            "#333333", // brightened from #000000 for dark terminals
	"blue":             "#5599ff", // brighter than CSS blue for visibility
	"brown":            "#aa5522",
	"burlywood":        "#ddbb88",
	"cadetblue":        "#5f9ea0",
	"chocolate":        "#d2691e",
	"coral":            "#ff7f50",
	"cyan":             "#00ffff",
	"darkblue":         "#2244aa",
	"darkfuscia":       "#993399",
	"darkgoldenrod":    "#b8860b",
	"darkgray":         "#a9a9a9",
	"darkgrey":         "#a9a9a9",
	"darkgreen":        "#006400",
	"darkorange":       "#ff8c00",
	"darkorchid":       "#9932cc",
	"darkred":          "#8b0000",
	"darkseagreen":     "#8fbc8f",
	"darkturquoise":    "#00ced1",
	"deepred":          "#cc0000",
	"firebrick":        "#b22222",
	"forestgreen":      "#228b22",
	"gold":             "#ffd700",
	"green":            "#00cc00", // brighter than CSS #008000
	"hotpink":          "#ff69b4",
	"indigo":           "#4b0082",
	"khaki":            "#f0e68c",
	"lawngreen":        "#7cfc00",
	"lightsteelblue":   "#b0c4de",
	"loveapple":        "#cc4444",
	"olivedrab":        "#6b8e23",
	"orange":           "#ffa500",
	"orangered":        "#ff4500",
	"peachpuff":        "#ffdab9",
	"peru":             "#cd853f",
	"powderblue":       "#b0e0e6",
	"purple":           "#aa00ff", // brighter than CSS #800080
	"red":              "#ff0000",
	"rosybrown":        "#bc8f8f",
	"seagreen":         "#2e8b57",
	"sienna":           "#a0522d",
	"slateblue":        "#6a5acd",
	"tan":              "#d2b48c", // standard CSS tan
	"thistle":          "#d8bfd8",
	"white":            "#ffffff",
	"yellowgreen":      "#9acd32",
}

// resolveColor converts a color attribute value to a hex color string.
// Handles both "#RRGGBB" hex format and named colors.
func resolveColor(c string) string {
	if strings.HasPrefix(c, "#") {
		return c
	}
	name := strings.ToLower(strings.TrimSpace(c))
	if hex, ok := namedColors[name]; ok {
		return hex
	}
	return ""
}

// brightenIfDark checks if a hex color is too dark for a dark terminal
// and brightens it to a minimum visible level. Returns the original color
// if it's bright enough, or a brightened version if it's too dark.
// Returns empty string for pure black (#000000) which the game uses as
// "default/no color".
func brightenIfDark(c string) string {
	raw := strings.TrimPrefix(c, "#")
	if len(raw) != 6 {
		return c
	}
	r := hexVal(raw[0])*16 + hexVal(raw[1])
	g := hexVal(raw[2])*16 + hexVal(raw[3])
	b := hexVal(raw[4])*16 + hexVal(raw[5])

	// Pure black = game's "default text" marker, skip it.
	if r == 0 && g == 0 && b == 0 {
		return ""
	}

	// Perceived brightness formula.
	brightness := (r*299 + g*587 + b*114) / 1000
	if brightness >= 40 {
		return c // bright enough
	}

	// Brighten: boost each channel so perceived brightness reaches ~50.
	// Blue-heavy colors (darkblue, indigo) need extra boost because
	// blue contributes little to perceived brightness.
	targetBrightness := 50.0
	scale := targetBrightness / float64(brightness)
	if scale < 1.5 {
		scale = 1.5 // minimum boost
	}

	// Also ensure at least one channel reaches 80+.
	maxCh := r
	if g > maxCh {
		maxCh = g
	}
	if b > maxCh {
		maxCh = b
	}
	if maxCh == 0 {
		return "#333333"
	}
	minScale := 80.0 / float64(maxCh)
	if minScale > scale {
		scale = minScale
	}

	nr := int(float64(r) * scale)
	ng := int(float64(g) * scale)
	nb := int(float64(b) * scale)
	if nr > 255 {
		nr = 255
	}
	if ng > 255 {
		ng = 255
	}
	if nb > 255 {
		nb = 255
	}

	hex := [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}
	return string([]byte{'#',
		hex[nr>>4], hex[nr&0xf],
		hex[ng>>4], hex[ng&0xf],
		hex[nb>>4], hex[nb&0xf],
	})
}

func hexVal(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return 0
	}
}
