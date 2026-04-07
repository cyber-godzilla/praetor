package colorwords

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cyber-godzilla/praetor/internal/types"
)

// ApplyColorWords scans styled segments for color word names and applies
// foreground coloring. Takes precedence over existing game styling.
func ApplyColorWords(segments []types.StyledSegment) []types.StyledSegment {
	var result []types.StyledSegment
	for _, seg := range segments {
		if seg.IsHR || seg.Text == "" {
			result = append(result, seg)
			continue
		}
		result = append(result, splitColorWords(seg)...)
	}
	return result
}

// splitColorWords splits a segment wherever a color phrase appears,
// applying the color to matched text including preceding adjectives.
func splitColorWords(seg types.StyledSegment) []types.StyledSegment {
	text := seg.Text
	lower := strings.ToLower(text)

	type match struct {
		start, end int
		color      string
		rainbow    bool      // per-character rainbow coloring (ROYGBIV)
		alternate  bool      // per-character alternating between two shades
		shades     [2]string // the two shades for alternation
	}
	var matches []match

	// Check for rainbow words first.
	for word := range rainbowWords {
		offset := 0
		for {
			idx := strings.Index(lower[offset:], word)
			if idx < 0 {
				break
			}
			start := offset + idx
			end := start + len(word)
			if start > 0 && isWordChar(rune(lower[start-1])) {
				offset = start + 1
				continue
			}
			if end < len(lower) && isWordChar(rune(lower[end])) {
				offset = end
				continue
			}
			adjStart := findAdjectiveStart(lower, start)
			matches = append(matches, match{start: adjStart, end: end, rainbow: true})
			offset = end
		}
	}

	// Check for plural color words first (per-character alternating shades).
	for plural, shades := range colorPluralMap {
		offset := 0
		for {
			idx := strings.Index(lower[offset:], plural)
			if idx < 0 {
				break
			}
			start := offset + idx
			end := start + len(plural)

			if start > 0 && isWordChar(rune(lower[start-1])) {
				offset = start + 1
				continue
			}
			if end < len(lower) && isWordChar(rune(lower[end])) {
				offset = end
				continue
			}

			adjStart := findAdjectiveStart(lower, start)
			matches = append(matches, match{start: adjStart, end: end, alternate: true, shades: shades})
			offset = end
		}
	}

	// Match single/multi-word phrases (sorted longest-first).
	for _, phrase := range sortedColorPhrases {
		color := colorWordMap[phrase]
		offset := 0
		for {
			idx := strings.Index(lower[offset:], phrase)
			if idx < 0 {
				break
			}
			start := offset + idx
			end := start + len(phrase)

			if start > 0 && isWordChar(rune(lower[start-1])) {
				offset = start + 1
				continue
			}

			// Check for color suffixes (e.g., "gold-tipped").
			suffixEnd := findColorSuffix(lower, end)
			if suffixEnd > end {
				end = suffixEnd
			} else if end < len(lower) && isWordChar(rune(lower[end])) {
				offset = end
				continue
			}

			adjStart := findAdjectiveStart(lower, start)
			matches = append(matches, match{start: adjStart, end: end, color: color})
			offset = end
		}
	}

	if len(matches) == 0 {
		return []types.StyledSegment{seg}
	}

	// Sort by start position, then longest match first for overlaps.
	for i := 1; i < len(matches); i++ {
		j := i
		for j > 0 && (matches[j].start < matches[j-1].start ||
			(matches[j].start == matches[j-1].start && matches[j].end > matches[j-1].end)) {
			matches[j], matches[j-1] = matches[j-1], matches[j]
			j--
		}
	}

	// Remove overlaps (keep longest/first).
	var filtered []match
	lastEnd := 0
	for _, m := range matches {
		if m.start >= lastEnd {
			filtered = append(filtered, m)
			lastEnd = m.end
		}
	}

	// Split into segments.
	var result []types.StyledSegment
	pos := 0
	for _, m := range filtered {
		if m.start > pos {
			result = append(result, types.StyledSegment{
				Text:      text[pos:m.start],
				Bold:      seg.Bold,
				Italic:    seg.Italic,
				Underline: seg.Underline,
				Color:     seg.Color,
			})
		}
		if m.rainbow {
			// Per-character rainbow coloring.
			matchText := text[m.start:m.end]
			ci := 0
			for _, ch := range matchText {
				result = append(result, types.StyledSegment{
					Text:      string(ch),
					Bold:      true,
					Italic:    seg.Italic,
					Underline: seg.Underline,
					Color:     rainbowColors[ci%len(rainbowColors)],
				})
				if ch != '-' && ch != ' ' {
					ci++
				}
			}
		} else if m.alternate {
			// Per-character alternating between two shades.
			matchText := text[m.start:m.end]
			ci := 0
			for _, ch := range matchText {
				result = append(result, types.StyledSegment{
					Text:      string(ch),
					Bold:      true,
					Italic:    seg.Italic,
					Underline: seg.Underline,
					Color:     brightenIfDark(m.shades[ci%2]),
				})
				if ch != '-' && ch != ' ' {
					ci++
				}
			}
		} else {
			result = append(result, types.StyledSegment{
				Text:      text[m.start:m.end],
				Bold:      true,
				Italic:    seg.Italic,
				Underline: seg.Underline,
				Color:     brightenIfDark(m.color),
			})
		}
		pos = m.end
	}
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

// findAdjectiveStart looks backwards from a color word to find preceding
// color adjectives (e.g., "glittering snow white" → start at "glittering").
func findAdjectiveStart(lower string, colorStart int) int {
	pos := colorStart
	for {
		candidate := pos
		for candidate > 0 && lower[candidate-1] == ' ' {
			candidate--
		}
		if candidate == pos {
			break
		}

		wordEnd := candidate
		wordStart := candidate
		for wordStart > 0 && isWordChar(rune(lower[wordStart-1])) {
			wordStart--
		}
		if wordStart == wordEnd {
			break
		}

		word := lower[wordStart:wordEnd]
		if !colorAdjectives[word] {
			break
		}

		pos = wordStart
	}
	return pos
}

// findColorSuffix checks if a known color suffix (e.g., "-tipped") starts at pos.
// Returns the end position past the suffix, or pos if no suffix found.
func findColorSuffix(lower string, pos int) int {
	for _, suffix := range colorSuffixes {
		if pos+len(suffix) <= len(lower) && lower[pos:pos+len(suffix)] == suffix {
			end := pos + len(suffix)
			// Make sure the suffix isn't followed by more word characters.
			if end < len(lower) && isWordChar(rune(lower[end])) {
				continue
			}
			return end
		}
	}
	return pos
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}

// brightenIfDark boosts a hex color if it's too dark for a dark terminal.
func brightenIfDark(c string) string {
	raw := strings.TrimPrefix(c, "#")
	if len(raw) != 6 {
		return c
	}
	r := hexByte(raw[0])*16 + hexByte(raw[1])
	g := hexByte(raw[2])*16 + hexByte(raw[3])
	b := hexByte(raw[4])*16 + hexByte(raw[5])

	brightness := (r*299 + g*587 + b*114) / 1000
	if brightness >= 40 {
		return c
	}
	if brightness == 0 {
		return "#555555"
	}

	scale := 50.0 / float64(brightness)
	if scale < 1.5 {
		scale = 1.5
	}
	maxCh := r
	if g > maxCh {
		maxCh = g
	}
	if b > maxCh {
		maxCh = b
	}
	if minScale := 80.0 / float64(maxCh); minScale > scale {
		scale = minScale
	}

	nr := min(int(float64(r)*scale), 255)
	ng := min(int(float64(g)*scale), 255)
	nb := min(int(float64(b)*scale), 255)
	return fmt.Sprintf("#%02x%02x%02x", nr, ng, nb)
}

func hexByte(b byte) int {
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
