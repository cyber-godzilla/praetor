package textutil

import "unicode/utf8"

// TrimLastRune returns s with its final UTF-8 rune removed, or "" if s is empty.
// Byte-wise truncation (s[:len(s)-1]) leaves invalid UTF-8 when erasing a
// multibyte character (accented letters, CJK, emoji); this removes a whole rune.
func TrimLastRune(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}
