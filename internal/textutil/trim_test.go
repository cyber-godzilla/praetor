package textutil

import "testing"

func TestTrimLastRune(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"a", ""},
		{"cafe", "caf"},
		{"café", "caf"}, // é is 2 bytes — must drop the whole rune
		{"日本", "日"},     // 3-byte runes
		{"ab😀", "ab"},   // 4-byte emoji
	}
	for _, tc := range cases {
		if got := TrimLastRune(tc.in); got != tc.want {
			t.Errorf("TrimLastRune(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
