// Package wildcard implements the user-facing wildcard language shared by
// highlights, custom views, notifications, speech colors, and ignorelists.
package wildcard

import (
	"regexp"
	"strings"
)

// Scope controls whether a pattern searches within a line or must consume an
// entire recognized name.
type Scope uint8

const (
	Substring Scope = iota
	Whole
)

// Span is a half-open byte range in the original UTF-8 text.
type Span struct {
	Start int
	End   int
}

// Pattern is an immutable compiled wildcard pattern.
type Pattern struct {
	re       *regexp.Regexp
	onlyStar bool
	empty    bool
}

// Compile translates *, ?, and their documented escapes into a
// case-insensitive regular expression. All other regular-expression syntax is
// literal. Star is deliberately non-greedy so a match at a given start uses
// the shortest text satisfying the pattern.
func Compile(pattern string, scope Scope) (Pattern, error) {
	if pattern == "" {
		return Pattern{empty: true}, nil
	}
	source := wildcardRegexp(pattern)
	if scope == Whole {
		source = "^(?:" + source + ")$"
	}
	re, err := regexp.Compile("(?i)" + source)
	if err != nil {
		return Pattern{}, err
	}
	return Pattern{re: re, onlyStar: pattern == "*"}, nil
}

// Match reports whether the pattern matches text under its compiled scope.
func (p Pattern) Match(text string) bool {
	return !p.empty && p.re != nil && p.re.MatchString(text)
}

// FindAll returns every non-overlapping match in left-to-right order. Empty
// matches are ignored to guarantee progress. A pattern consisting only of '*'
// is the documented exception and selects the complete nonempty line.
func (p Pattern) FindAll(text string) []Span {
	if p.empty || p.re == nil {
		return nil
	}
	if p.onlyStar {
		if text == "" {
			return nil
		}
		return []Span{{Start: 0, End: len(text)}}
	}
	indices := p.re.FindAllStringIndex(text, -1)
	spans := make([]Span, 0, len(indices))
	for _, index := range indices {
		if index[0] == index[1] {
			continue
		}
		spans = append(spans, Span{Start: index[0], End: index[1]})
	}
	return spans
}

func wildcardRegexp(pattern string) string {
	var out strings.Builder
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '*' || next == '?' || next == '\\' {
				out.WriteString(regexp.QuoteMeta(string(next)))
				i++
				continue
			}
		}
		switch r {
		case '*':
			out.WriteString(".*?")
		case '?':
			out.WriteString(".")
		default:
			out.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	return out.String()
}
