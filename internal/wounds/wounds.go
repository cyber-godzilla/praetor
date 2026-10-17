// Package wounds recognizes the verified Eternal City wound taxonomy and
// applies category/severity styling without changing the underlying text.
package wounds

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

type Category string

const (
	Cuts      Category = "cuts"
	Bruises   Category = "bruises"
	Punctures Category = "punctures"
	Burns     Category = "burns"
)

type Match struct {
	Start    int
	End      int
	Phrase   string
	Category Category
	Severity int
}

type phraseSpec struct {
	text     string
	category Category
	severity int
}

// Longest phrases come first so a canonical phrase such as "deep cut" wins
// over its nested generic noun "cut".
var taxonomy = []phraseSpec{
	{"devastating cut", Cuts, 5},
	{"severe puncture", Punctures, 4},
	{"moderate burn", Burns, 3},
	{"deep puncture", Punctures, 3},
	{"severe burn", Burns, 4},
	{"shallow cut", Cuts, 1},
	{"severe cut", Cuts, 4},
	{"small bruise", Bruises, 1},
	{"ugly bruise", Bruises, 3},
	{"major bruise", Bruises, 4},
	{"faint wound", Punctures, 1},
	{"gaping wound", Punctures, 5},
	{"minor burn", Burns, 1},
	{"deep cut", Cuts, 3},
	{"puncture", Punctures, 2},
	{"fracture", Bruises, 5},
	{"bruise", Bruises, 2},
	{"burn", Burns, 2},
	{"cut", Cuts, 2},
}

type inspectionSpec struct {
	phrase   string
	link     string
	category Category
	severity int
}

// inspectionTaxonomy is the character-appearance vocabulary observed in
// retained game session logs. Severity is provisional: it preserves the
// apparent least-to-most-serious ordering within each family until every tier
// can be correlated with a controlled combat or healer observation.
//
// Unknown cutting/piercing tiers and every burning description intentionally
// have no executable placeholder. Matching an invented phrase would be worse
// than leaving an as-yet-unverified wound unstyled.
var inspectionTaxonomy = []inspectionSpec{
	{"completely black and blue", "is ", Bruises, 5},
	{"large black bruise", "has a ", Bruises, 3},
	{"badly punctured", "is ", Punctures, 2},
	{"badly scratched", "is ", Cuts, 2},
	{"slashed open", "is ", Cuts, 4},
	{"large bruise", "has a ", Bruises, 2},
	{"badly bruised", "is ", Bruises, 4},
	{"punctured", "is ", Punctures, 1},
	{"scratched", "is ", Cuts, 1},
	{"slashed", "is ", Cuts, 3},
	{"pierced", "is ", Punctures, 3},
	{"bruised", "is ", Bruises, 1},
}

var inspectionBodyParts = []string{
	"head", "face", "neck", "chest", "back", "waist", "groin",
	"left shoulder", "right shoulder", "left arm", "right arm",
	"left hand", "right hand", "left thigh", "right thigh",
	"left shin", "right shin", "left leg", "right leg",
	"left foot", "right foot",
}

// Find returns verified wound phrases from combat-result noun phrases ("a
// cut", "an ugly bruise") and character-inspection sentences ("Her face is
// slashed"). Requiring the surrounding grammar keeps ambiguous verbs such as
// "cut the rope" and damaged equipment unchanged. Unknown, plural, treatment,
// and bleeding descriptions are deliberately outside the current catalogue.
func Find(text string) []Match {
	lower := lowerASCII(text)
	var matches []Match
	for _, spec := range taxonomy {
		from := 0
		for from < len(lower) {
			rel := strings.Index(lower[from:], spec.text)
			if rel < 0 {
				break
			}
			start := from + rel
			end := start + len(spec.text)
			from = start + 1
			if !wordBoundary(text, start, end) || !hasIndefiniteArticle(text, start) || overlaps(matches, start, end) {
				continue
			}
			matches = append(matches, Match{
				Start: start, End: end, Phrase: text[start:end],
				Category: spec.category, Severity: spec.severity,
			})
		}
	}
	for _, spec := range inspectionTaxonomy {
		from := 0
		for from < len(lower) {
			rel := strings.Index(lower[from:], spec.phrase)
			if rel < 0 {
				break
			}
			start := from + rel
			end := start + len(spec.phrase)
			from = start + 1
			if !wordBoundary(text, start, end) || !hasInspectionContext(lower, start, end, spec.link) || overlaps(matches, start, end) {
				continue
			}
			matches = append(matches, Match{
				Start: start, End: end, Phrase: text[start:end],
				Category: spec.category, Severity: spec.severity,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Start < matches[j].Start })
	return matches
}

func hasInspectionContext(lower string, start, end int, link string) bool {
	if end >= len(lower) || lower[end] != '.' {
		return false
	}
	prefix := lower[:start]
	for _, pronoun := range []string{"his ", "her "} {
		for _, part := range inspectionBodyParts {
			context := pronoun + part + " " + link
			contextStart := len(prefix) - len(context)
			if contextStart < 0 || !strings.HasSuffix(prefix, context) {
				continue
			}
			if contextStart == 0 {
				return true
			}
			r, _ := utf8.DecodeLastRuneInString(prefix[:contextStart])
			if !isWord(r) {
				return true
			}
		}
	}
	return false
}

// Apply overlays wound foreground colors on the original styled segments.
// Segment boundaries and all unrelated attributes are preserved; levels 4-5
// add bold without removing pre-existing emphasis.
func Apply(segments []types.StyledSegment, cfg config.WoundHighlightConfig) []types.StyledSegment {
	if !cfg.Enabled || len(segments) == 0 {
		return segments
	}
	var text strings.Builder
	for _, seg := range segments {
		text.WriteString(seg.Text)
	}
	matches := Find(text.String())
	if len(matches) == 0 {
		return segments
	}

	result := make([]types.StyledSegment, 0, len(segments)+len(matches)*2)
	position := 0
	matchIndex := 0
	for _, seg := range segments {
		segStart := position
		segEnd := segStart + len(seg.Text)
		position = segEnd
		if seg.IsHR || seg.Text == "" {
			result = append(result, seg)
			continue
		}
		for matchIndex < len(matches) && matches[matchIndex].End <= segStart {
			matchIndex++
		}
		cursor := segStart
		idx := matchIndex
		for cursor < segEnd {
			if idx >= len(matches) || matches[idx].Start >= segEnd {
				result = append(result, slice(seg, text.String()[cursor:segEnd]))
				break
			}
			match := matches[idx]
			if match.End <= cursor {
				idx++
				continue
			}
			if match.Start > cursor {
				end := min(match.Start, segEnd)
				result = append(result, slice(seg, text.String()[cursor:end]))
				cursor = end
				continue
			}
			end := min(match.End, segEnd)
			styled := slice(seg, text.String()[cursor:end])
			family := familyFor(cfg, match.Category)
			if color, ok := FamilyColor(family, match.Severity); ok {
				styled.Color = color
				styled.Bold = styled.Bold || match.Severity >= 4
			}
			result = append(result, styled)
			cursor = end
			if cursor >= match.End {
				idx++
			}
		}
		matchIndex = idx
	}
	return result
}

var familyShades = map[string][5]string{
	"red":    {"#7f1d1d", "#b91c1c", "#dc2626", "#ef4444", "#f87171"},
	"purple": {"#581c87", "#7e22ce", "#9333ea", "#a855f7", "#c084fc"},
	"blue":   {"#1e3a8a", "#1d4ed8", "#2563eb", "#3b82f6", "#60a5fa"},
	"orange": {"#7c2d12", "#c2410c", "#ea580c", "#f97316", "#fb923c"},
}

func FamilyColor(family string, severity int) (string, bool) {
	shades, ok := familyShades[family]
	if !ok || severity < 1 || severity > len(shades) {
		return "", false
	}
	return shades[severity-1], true
}

func familyFor(cfg config.WoundHighlightConfig, category Category) string {
	switch category {
	case Cuts:
		return cfg.Cuts
	case Bruises:
		return cfg.Bruises
	case Punctures:
		return cfg.Punctures
	case Burns:
		return cfg.Burns
	default:
		return ""
	}
}

func lowerASCII(value string) string {
	data := []byte(value)
	for i, b := range data {
		if b >= 'A' && b <= 'Z' {
			data[i] = b + ('a' - 'A')
		}
	}
	return string(data)
}

func wordBoundary(text string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if isWord(r) {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if isWord(r) {
			return false
		}
	}
	return true
}

func hasIndefiniteArticle(text string, start int) bool {
	prefix := strings.TrimRightFunc(text[:start], unicode.IsSpace)
	end := len(prefix)
	for len(prefix) > 0 {
		r, size := utf8.DecodeLastRuneInString(prefix)
		if !unicode.IsLetter(r) {
			break
		}
		prefix = prefix[:len(prefix)-size]
	}
	article := lowerASCII(text[len(prefix):end])
	return article == "a" || article == "an"
}

func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func overlaps(matches []Match, start, end int) bool {
	for _, match := range matches {
		if start < match.End && end > match.Start {
			return true
		}
	}
	return false
}

func slice(seg types.StyledSegment, text string) types.StyledSegment {
	seg.Text = text
	return seg
}
