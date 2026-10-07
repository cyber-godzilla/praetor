package engine

import (
	"testing"
)

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		want    bool
	}{
		{"You take", false},
		{"You take * sword", true},
		{"You take ? sword", true},
		{"*", true},
		{"?", true},
		{"plain text", false},
		{"", false},
		{"You * at * with *", true},
	}

	for _, tt := range tests {
		got := isWildcard(tt.pattern)
		if got != tt.want {
			t.Errorf("isWildcard(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

func TestMatcher_SubstringMatch(t *testing.T) {
	m := NewMatcher()

	cp := m.Compile("You take")
	if !m.Match(cp, "You take a bronze sword from the corpse") {
		t.Error("expected substring match for 'You take' in text")
	}
	if !m.Match(cp, "You take") {
		t.Error("expected exact substring match")
	}
	if m.Match(cp, "You drop a sword") {
		t.Error("expected no match for unrelated text")
	}
	if m.Match(cp, "") {
		t.Error("expected no match for empty text")
	}
}

func TestMatcher_WildcardStar(t *testing.T) {
	m := NewMatcher()

	cp := m.Compile("You take * sword")
	if !m.Match(cp, "You take a sword") {
		t.Error("expected match: 'You take a sword'")
	}
	if !m.Match(cp, "You take a bronze sword") {
		t.Error("expected match: 'You take a bronze sword'")
	}
	if m.Match(cp, "You take a shield") {
		t.Error("expected no match: 'You take a shield'")
	}

	// Multiple wildcards
	cp2 := m.Compile("You * at * with *")
	if !m.Match(cp2, "You slash at the bandit with your sword") {
		t.Error("expected multi-wildcard match")
	}
	if m.Match(cp2, "You slash the bandit") {
		t.Error("expected no match without 'at' and 'with'")
	}
}

func TestMatcher_WildcardQuestion(t *testing.T) {
	m := NewMatcher()

	cp := m.Compile("You take ? sword")
	if !m.Match(cp, "You take a sword") {
		t.Error("expected match: single char 'a'")
	}
	if !m.Match(cp, "You take 1 sword") {
		t.Error("expected match: single char '1'")
	}
	if m.Match(cp, "You take an sword") {
		t.Error("expected no match: 'an' is two chars")
	}
}

func TestMatcher_Anchors(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{name: "literal start match", pattern: "^You attack", text: "You attack the bandit", want: true},
		{name: "literal start rejects embedded", pattern: "^You attack", text: "A bandit says: You attack poorly", want: false},
		{name: "literal end match", pattern: "leaves.$", text: "The bandit leaves.", want: true},
		{name: "literal end rejects trailing text", pattern: "leaves.$", text: "The bandit leaves. Suddenly", want: false},
		{name: "literal exact match", pattern: "^You are ready.$", text: "You are ready.", want: true},
		{name: "literal exact rejects prefix", pattern: "^You are ready.$", text: "Now You are ready.", want: false},
		{name: "literal exact rejects suffix", pattern: "^You are ready.$", text: "You are ready. Go", want: false},
		{name: "wildcard start match", pattern: "^You can't *", text: "You can't attack the corpse", want: true},
		{name: "wildcard start rejects quoted text", pattern: "^You can't *", text: `A marauder laughs, "You can't penetrate my defenses."`, want: false},
		{name: "wildcard end match", pattern: "You * the corpse$", text: "You slash the corpse", want: true},
		{name: "wildcard end rejects suffix", pattern: "You * the corpse$", text: "You slash the corpse again", want: false},
		{name: "wildcard exact match", pattern: "^You * the corpse$", text: "You slash the corpse", want: true},
		{name: "wildcard exact rejects prefix", pattern: "^You * the corpse$", text: "Then You slash the corpse", want: false},
		{name: "internal caret remains literal", pattern: "rank ^ bonus", text: "Your rank ^ bonus rises", want: true},
		{name: "internal dollar remains literal", pattern: "cost $5 today", text: "The cost $5 today", want: true},
		{name: "empty exact pattern", pattern: "^$", text: "", want: true},
		{name: "empty exact rejects text", pattern: "^$", text: "anything", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.Match(m.Compile(tt.pattern), tt.text); got != tt.want {
				t.Errorf("Match(Compile(%q), %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
			}
		})
	}
}

func TestMatcher_RegexCharsEscaped(t *testing.T) {
	m := NewMatcher()

	// Parens should be treated as literal
	cp := m.Compile("(copper)")
	if !m.Match(cp, "You see (copper) coins on the ground") {
		t.Error("expected literal paren match")
	}
	if m.Match(cp, "You see copper coins on the ground") {
		t.Error("expected no match without parens")
	}

	// Brackets should be treated as literal
	cp2 := m.Compile("[rare]")
	if !m.Match(cp2, "You found a [rare] gem") {
		t.Error("expected literal bracket match")
	}
	if m.Match(cp2, "You found a rare gem") {
		t.Error("expected no match without brackets")
	}
}

func TestMatcher_RegexCharsEscapedWithWildcard(t *testing.T) {
	m := NewMatcher()

	// Wildcard pattern with regex-special chars
	cp := m.Compile("(copper) * coins")
	if !m.Match(cp, "(copper) shiny coins") {
		t.Error("expected match with wildcard and escaped parens")
	}
}

func TestMatcher_MatchAny(t *testing.T) {
	m := NewMatcher()

	patterns := []CompiledPattern{
		m.Compile("You take"),
		m.Compile("You don't see"),
		m.Compile("You * at * with *"),
	}

	idx := m.MatchAny(patterns, "You take a sword")
	if idx != 0 {
		t.Errorf("MatchAny got %d, want 0", idx)
	}

	idx = m.MatchAny(patterns, "You don't see that here")
	if idx != 1 {
		t.Errorf("MatchAny got %d, want 1", idx)
	}

	idx = m.MatchAny(patterns, "You slash at the goblin with a blade")
	if idx != 2 {
		t.Errorf("MatchAny got %d, want 2", idx)
	}

	idx = m.MatchAny(patterns, "Nothing matches here")
	if idx != -1 {
		t.Errorf("MatchAny got %d, want -1", idx)
	}

	// Empty patterns list
	idx = m.MatchAny(nil, "some text")
	if idx != -1 {
		t.Errorf("MatchAny on nil got %d, want -1", idx)
	}
}

func TestMatcher_CacheReuse(t *testing.T) {
	m := NewMatcher()

	cp1 := m.Compile("You take")
	cp2 := m.Compile("You take")

	// Both should produce the same result
	if cp1 != cp2 {
		t.Error("expected cached pattern to be identical")
	}
}

func TestMatcher_ClearCache(t *testing.T) {
	m := NewMatcher()
	m.Compile("You take")
	m.Compile("You drop")

	m.ClearCache()

	m.mu.RLock()
	cacheLen := len(m.cache)
	m.mu.RUnlock()

	if cacheLen != 0 {
		t.Errorf("expected empty cache after ClearCache, got %d entries", cacheLen)
	}
}

func TestMatcher_EmptyPattern(t *testing.T) {
	m := NewMatcher()

	cp := m.Compile("")
	// Empty pattern should match everything via substring (strings.Contains returns true for "")
	if !m.Match(cp, "anything") {
		t.Error("expected empty pattern to match any text")
	}
}
