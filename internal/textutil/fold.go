package textutil

// ToLowerASCII lowercases only ASCII A–Z, leaving every other byte untouched.
//
// Unlike strings.ToLower it is length-preserving by construction: some Unicode
// case folds change byte length (Ⱥ U+023A folds 2→3 bytes, İ U+0130 folds
// 2→1), so byte offsets computed against strings.ToLower(s) are not valid
// indexes into s. Callers that match ASCII patterns and then slice the original
// string by the match offsets (colorwords, string highlights) must fold with
// this to avoid slice-bounds panics and mid-rune splits.
func ToLowerASCII(s string) string {
	// Fast path: nothing to fold (common for already-lowercase game text).
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
