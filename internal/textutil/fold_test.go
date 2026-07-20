package textutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestToLowerASCII_LowercasesOnlyASCII(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"ABC", "abc"},
		{"AbC123", "abc123"},
		{"Gold Ring", "gold ring"},
		// Non-ASCII letters are left exactly as-is (not folded).
		{"Ⱥbc", "Ⱥbc"},
		{"İ", "İ"},
		{"CRIMSON café", "crimson café"},
	}
	for _, c := range cases {
		if got := ToLowerASCII(c.in); got != c.want {
			t.Errorf("ToLowerASCII(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The defining property: the fold never changes byte length, so offsets found
// in the folded string are valid indexes into the original.
func TestToLowerASCII_PreservesByteLength(t *testing.T) {
	for _, s := range []string{"", "abc", "ABC", "ȺȺȺȺred", "İİİİ crimson", "café", "𝔘𝔫𝔦", "MiXeD Ⱥ İ"} {
		if got := ToLowerASCII(s); len(got) != len(s) {
			t.Errorf("ToLowerASCII(%q): len %d != original len %d", s, len(got), len(s))
		}
	}
}

// It agrees with strings.ToLower on pure-ASCII input.
func TestToLowerASCII_MatchesStdlibOnASCII(t *testing.T) {
	for _, s := range []string{"Hello World", "GOLD", "Retalq Blade", "aB9_-"} {
		if got, want := ToLowerASCII(s), strings.ToLower(s); got != want {
			t.Errorf("ToLowerASCII(%q) = %q, strings.ToLower = %q", s, got, want)
		}
	}
}

// Offsets from the folded string must slice the original without tearing runes.
func TestToLowerASCII_OffsetsSliceOriginalSafely(t *testing.T) {
	text := "ȺȺȺȺred"
	lower := ToLowerASCII(text)
	idx := strings.Index(lower, "red")
	if idx < 0 {
		t.Fatal("expected to find 'red'")
	}
	got := text[idx : idx+len("red")]
	if got != "red" || !utf8.ValidString(text[:idx]) {
		t.Errorf("slice by folded offset = %q (valid prefix=%v), want clean 'red'", got, utf8.ValidString(text[:idx]))
	}
}
