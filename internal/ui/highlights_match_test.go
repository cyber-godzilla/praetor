package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

// TestApplyHighlights_CaseFoldingRunesNoPanic is a regression guard: match
// offsets were computed against strings.ToLower(text) but used to slice the
// original text, and `end` mixed len(original pattern) with an index in the
// lowered text. Runes whose lowercase changes byte length (Ⱥ 2→3, İ 2→1)
// panicked with slice-bounds-out-of-range — fires whenever any highlight is
// configured, so it is player-triggerable via a say containing such runes.
func TestApplyHighlights_CaseFoldingRunesNoPanic(t *testing.T) {
	hl := []config.HighlightConfig{{Pattern: "loot", Style: "gold", Active: true}}
	for _, text := range []string{"ȺȺȺȺloot", "İİİİ loot", "a LOOT chest", "Ⱥ retalq blade"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("applyHighlights panicked on %q: %v", text, r)
				}
			}()
			out := applyHighlights([]types.StyledSegment{{Text: text}}, hl)
			var sb strings.Builder
			for _, s := range out {
				sb.WriteString(s.Text)
			}
			if sb.String() != text {
				t.Errorf("segments for %q reassemble to %q", text, sb.String())
			}
			for _, s := range out {
				if !utf8.ValidString(s.Text) {
					t.Errorf("segment %q is not valid UTF-8 (torn rune) from input %q", s.Text, text)
				}
			}
		}()
	}
}

// A lowercase pattern must still match uppercase text and vice versa, with the
// highlighted run landing on the right characters even after a length-changing
// rune shifts byte positions.
func TestApplyHighlights_CaseInsensitiveSpan(t *testing.T) {
	hl := []config.HighlightConfig{{Pattern: "LOOT", Style: "gold", Active: true}}
	out := applyHighlights([]types.StyledSegment{{Text: "ȺȺȺȺloot here"}}, hl)
	found := false
	for _, s := range out {
		if s.Text == "loot" && s.Color == "highlight:gold" {
			found = true
		}
	}
	if !found {
		t.Errorf("mixed-case 'loot' not highlighted after a growing rune: %+v", out)
	}
}
