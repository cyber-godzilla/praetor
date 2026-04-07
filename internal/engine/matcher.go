package engine

import (
	"regexp"
	"strings"
	"sync"
)

// CompiledPattern holds a pre-compiled pattern for matching game text.
type CompiledPattern struct {
	literal string         // non-empty for substring match
	regex   *regexp.Regexp // non-nil for wildcard match
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
// if available. Patterns without * or ? use substring matching. Patterns with
// wildcards are compiled to regular expressions.
func (m *Matcher) Compile(pattern string) CompiledPattern {
	m.mu.RLock()
	if cp, ok := m.cache[pattern]; ok {
		m.mu.RUnlock()
		return cp
	}
	m.mu.RUnlock()

	var cp CompiledPattern
	if !isWildcard(pattern) {
		cp = CompiledPattern{literal: pattern}
	} else {
		cp = CompiledPattern{regex: compileWildcard(pattern)}
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

// compileWildcard converts a wildcard pattern to a regexp. Special regex
// characters are escaped, then * becomes .* and ? becomes a single-char match.
func compileWildcard(pattern string) *regexp.Regexp {
	// Escape all regex metacharacters first
	escaped := regexp.QuoteMeta(pattern)
	// Now convert our wildcard placeholders (which were escaped)
	// QuoteMeta turns * into \* and ? into \?
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	return regexp.MustCompile(escaped)
}
