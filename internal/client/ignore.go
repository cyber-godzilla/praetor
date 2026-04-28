package client

import (
	"regexp"
	"strings"
	"sync"
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
	ooc    map[string]struct{}
	think  map[string]struct{}
	oocRe  *regexp.Regexp
	thnkRe *regexp.Regexp
}

// NewIgnoreFilter returns an empty filter with the regexes compiled.
func NewIgnoreFilter() *IgnoreFilter {
	return &IgnoreFilter{
		ooc:    map[string]struct{}{},
		think:  map[string]struct{}{},
		oocRe:  regexp.MustCompile(`^<\d{1,2}:\d{2}\s*[apAP][mM]\s+OOC>\s+([A-Za-z][A-Za-z0-9'\-]*)\b`),
		thnkRe: regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9'\-]*)\s+thinks aloud:`),
	}
}

// SetOOC replaces the OOC ignorelist with the given account names.
// Names are stored lowercased for case-insensitive lookup.
func (f *IgnoreFilter) SetOOC(names []string) {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		m[strings.ToLower(n)] = struct{}{}
	}
	f.mu.Lock()
	f.ooc = m
	f.mu.Unlock()
}

// SetThink replaces the Think ignorelist with the given character names.
func (f *IgnoreFilter) SetThink(names []string) {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		m[strings.ToLower(n)] = struct{}{}
	}
	f.mu.Lock()
	f.think = m
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
			if _, hit := f.ooc[strings.ToLower(m[1])]; hit {
				return IgnoreChannelOOC, m[1], true
			}
		}
	}
	if len(f.think) > 0 {
		if m := f.thnkRe.FindStringSubmatch(text); len(m) == 2 {
			if _, hit := f.think[strings.ToLower(m[1])]; hit {
				return IgnoreChannelThink, m[1], true
			}
		}
	}
	return IgnoreChannelNone, "", false
}

// ShouldDrop is a temporary shim during the v2 refactor. Task 6 will
// remove it once handleGameText is migrated to Match.
func (f *IgnoreFilter) ShouldDrop(text string) bool {
	_, _, hit := f.Match(text)
	return hit
}
