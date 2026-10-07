package engine

import (
	"regexp"
	"strings"
	"sync"
)

// CompiledPattern holds a pre-compiled pattern for matching game text.
type CompiledPattern struct {
	literal     string         // used for literal substring/prefix/suffix/exact matches
	regex       *regexp.Regexp // non-nil for wildcard match
	anchorStart bool
	anchorEnd   bool
}

// Matcher compiles and caches patterns for matching game text.
type Matcher struct {
	mu    sync.RWMutex
	cache map[string]CompiledPattern
}

// NewMatcher creates a new Matcher with an empty cache.
func NewMatcher() *Matcher {
	return &Matcher{
		cache: make(map[string]CompiledPattern),
	}
}

// Compile converts a pattern string into a CompiledPattern, using the cache
// if available. Patterns without * or ? use literal matching. Patterns with
// wildcards are compiled to regular expressions. A leading ^ or trailing $
// anchors either form to the corresponding line boundary.
func (m *Matcher) Compile(pattern string) CompiledPattern {
	m.mu.RLock()
	if cp, ok := m.cache[pattern]; ok {
		m.mu.RUnlock()
		return cp
	}
	m.mu.RUnlock()

	matchPattern, anchorStart, anchorEnd := splitAnchors(pattern)

	var cp CompiledPattern
	if !isWildcard(pattern) {
		cp = CompiledPattern{
			literal:     matchPattern,
			anchorStart: anchorStart,
			anchorEnd:   anchorEnd,
		}
	} else {
		cp = CompiledPattern{regex: compileWildcard(matchPattern, anchorStart, anchorEnd)}
	}

	m.mu.Lock()
	m.cache[pattern] = cp
	m.mu.Unlock()

	return cp
}

// Match tests whether a compiled pattern matches the given text.
func (m *Matcher) Match(cp CompiledPattern, text string) bool {
	if cp.regex != nil {
		return cp.regex.MatchString(text)
	}
	if cp.anchorStart && cp.anchorEnd {
		return text == cp.literal
	}
	if cp.anchorStart {
		return strings.HasPrefix(text, cp.literal)
	}
	if cp.anchorEnd {
		return strings.HasSuffix(text, cp.literal)
	}
	return strings.Contains(text, cp.literal)
}

// MatchAny tests patterns in order and returns the index of the first match,
// or -1 if none match.
func (m *Matcher) MatchAny(patterns []CompiledPattern, text string) int {
	for i, cp := range patterns {
		if m.Match(cp, text) {
			return i
		}
	}
	return -1
}

// ClearCache removes all cached compiled patterns.
func (m *Matcher) ClearCache() {
	m.mu.Lock()
	m.cache = make(map[string]CompiledPattern)
	m.mu.Unlock()
}

// isWildcard returns true if the pattern contains * or ? wildcard characters.
func isWildcard(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

// splitAnchors removes the optional boundary markers from a pattern. Carets
// and dollar signs anywhere else remain ordinary literal characters.
func splitAnchors(pattern string) (matchPattern string, anchorStart, anchorEnd bool) {
	matchPattern = pattern
	if strings.HasPrefix(matchPattern, "^") {
		anchorStart = true
		matchPattern = strings.TrimPrefix(matchPattern, "^")
	}
	if strings.HasSuffix(matchPattern, "$") {
		anchorEnd = true
		matchPattern = strings.TrimSuffix(matchPattern, "$")
	}
	return matchPattern, anchorStart, anchorEnd
}

// compileWildcard converts a wildcard pattern to a regexp. Special regex
// characters are escaped, then * becomes .* and ? becomes a single-char match.
// Optional line-boundary anchors are added after escaping so they retain their
// regex meaning while carets and dollar signs inside the pattern stay literal.
func compileWildcard(pattern string, anchorStart, anchorEnd bool) *regexp.Regexp {
	// Escape all regex metacharacters first
	escaped := regexp.QuoteMeta(pattern)
	// Now convert our wildcard placeholders (which were escaped)
	// QuoteMeta turns * into \* and ? into \?
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	if anchorStart {
		escaped = "^" + escaped
	}
	if anchorEnd {
		escaped += "$"
	}
	return regexp.MustCompile(escaped)
}
