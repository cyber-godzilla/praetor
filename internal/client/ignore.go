package client

import (
	"regexp"
	"strings"
	"sync"

	sharedvars "github.com/cyber-godzilla/praetor/internal/variables"
	"github.com/cyber-godzilla/praetor/internal/wildcard"
)

// IgnoreChannel identifies which channel matched in IgnoreFilter.Match.
type IgnoreChannel int

const (
	IgnoreChannelNone IgnoreChannel = iota
	IgnoreChannelOOC
	IgnoreChannelThink
)

// String returns a short channel label suitable for placeholder text.
func (c IgnoreChannel) String() string {
	switch c {
	case IgnoreChannelOOC:
		return "OOC"
	case IgnoreChannelThink:
		return "think"
	default:
		return ""
	}
}

// IgnoreFilter suppresses game-text lines that originate from listed
// accounts (OOC channel) or characters (think aloud). The two lists
// are independent — an entry on one does not affect the other.
type IgnoreFilter struct {
	mu     sync.RWMutex
	ooc    []string
	think  []string
	values map[string]string
	oocRe  *regexp.Regexp
	thnkRe *regexp.Regexp
}

// NewIgnoreFilter returns an empty filter with the regexes compiled.
func NewIgnoreFilter() *IgnoreFilter {
	return &IgnoreFilter{
		values: map[string]string{},
		oocRe:  regexp.MustCompile(`^<\d{1,2}:\d{2}\s*[apAP][mM]\s+OOC>\s+([A-Za-z][A-Za-z0-9'\-]*)\b`),
		thnkRe: regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9'\-]*)\s+thinks aloud:`),
	}
}

// SetOOC replaces the OOC ignorelist with whole-name wildcard patterns.
func (f *IgnoreFilter) SetOOC(names []string) {
	f.mu.Lock()
	f.ooc = cleanPatterns(names)
	f.mu.Unlock()
}

// SetThink replaces the Think ignorelist with whole-name wildcard patterns.
func (f *IgnoreFilter) SetThink(names []string) {
	f.mu.Lock()
	f.think = cleanPatterns(names)
	f.mu.Unlock()
}

// SetVariables replaces the live variable snapshot used when matching list
// entries. Patterns are expanded at match time so edits take effect without
// rebuilding the lists.
func (f *IgnoreFilter) SetVariables(values map[string]string) {
	cloned := make(map[string]string, len(values))
	for name, value := range values {
		cloned[name] = value
	}
	f.mu.Lock()
	f.values = cloned
	f.mu.Unlock()
}

// Match runs both regexes against the given parsed-text line. On a hit,
// it returns the matching channel and the case-preserved captured name.
// On miss, returns (IgnoreChannelNone, "", false).
func (f *IgnoreFilter) Match(text string) (IgnoreChannel, string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.ooc) > 0 {
		if m := f.oocRe.FindStringSubmatch(text); len(m) == 2 {
			if matchesIgnorePattern(f.ooc, m[1], f.values) {
				return IgnoreChannelOOC, m[1], true
			}
		}
	}
	if len(f.think) > 0 {
		if m := f.thnkRe.FindStringSubmatch(text); len(m) == 2 {
			if matchesIgnorePattern(f.think, m[1], f.values) {
				return IgnoreChannelThink, m[1], true
			}
		}
	}
	return IgnoreChannelNone, "", false
}

func cleanPatterns(names []string) []string {
	patterns := make([]string, 0, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			patterns = append(patterns, name)
		}
	}
	return patterns
}

func matchesIgnorePattern(patterns []string, name string, values map[string]string) bool {
	for _, raw := range patterns {
		expanded, err := sharedvars.Expand(raw, values)
		if err != nil || expanded == "" {
			continue
		}
		pattern, err := wildcard.Compile(expanded, wildcard.Whole)
		if err == nil && pattern.Match(name) {
			return true
		}
	}
	return false
}
