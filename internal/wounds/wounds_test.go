package wounds

import (
	"reflect"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestFindVerifiedWoundTaxonomy(t *testing.T) {
	tests := []struct {
		phrase   string
		category Category
		severity int
	}{
		{"shallow cut", Cuts, 1},
		{"cut", Cuts, 2},
		{"deep cut", Cuts, 3},
		{"severe cut", Cuts, 4},
		{"devastating cut", Cuts, 5},
		{"small bruise", Bruises, 1},
		{"bruise", Bruises, 2},
		{"ugly bruise", Bruises, 3},
		{"major bruise", Bruises, 4},
		{"fracture", Bruises, 5},
		{"faint wound", Punctures, 1},
		{"puncture", Punctures, 2},
		{"deep puncture", Punctures, 3},
		{"severe puncture", Punctures, 4},
		{"gaping wound", Punctures, 5},
		{"minor burn", Burns, 1},
		{"burn", Burns, 2},
		{"moderate burn", Burns, 3},
		{"severe burn", Burns, 4},
	}

	for _, test := range tests {
		t.Run(test.phrase, func(t *testing.T) {
			article := "a"
			if test.phrase == "ugly bruise" {
				article = "an"
			}
			text := "She has " + article + " " + test.phrase + " on her left arm."
			matches := Find(text)
			if len(matches) != 1 {
				t.Fatalf("Find(%q) returned %d matches: %+v", text, len(matches), matches)
			}
			got := matches[0]
			if got.Phrase != test.phrase || got.Category != test.category || got.Severity != test.severity {
				t.Fatalf("Find(%q) = %+v", text, got)
			}
			if text[got.Start:got.End] != test.phrase {
				t.Fatalf("span %d:%d = %q", got.Start, got.End, text[got.Start:got.End])
			}
		})
	}
}

func TestFindVerifiedCharacterInspectionCatalog(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		phrase   string
		category Category
		severity int
	}{
		{"scratched", "Her waist is scratched.", "scratched", Cuts, 1},
		{"badly scratched", "His right arm is badly scratched.", "badly scratched", Cuts, 2},
		{"slashed", "Her chest is slashed.", "slashed", Cuts, 3},
		{"slashed open", "Her face is slashed open.", "slashed open", Cuts, 4},
		{"punctured", "His left thigh is punctured.", "punctured", Punctures, 1},
		{"badly punctured", "Her right hand is badly punctured.", "badly punctured", Punctures, 2},
		{"pierced", "His chest is pierced.", "pierced", Punctures, 3},
		{"bruised", "Her face is bruised.", "bruised", Bruises, 1},
		{"large bruise", "His right shoulder has a large bruise.", "large bruise", Bruises, 2},
		{"large black bruise", "Her left shin has a large black bruise.", "large black bruise", Bruises, 3},
		{"badly bruised", "His head is badly bruised.", "badly bruised", Bruises, 4},
		{"black and blue", "Her face is completely black and blue.", "completely black and blue", Bruises, 5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := Find(test.text)
			if len(matches) != 1 {
				t.Fatalf("Find(%q) returned %d matches: %+v", test.text, len(matches), matches)
			}
			got := matches[0]
			if got.Phrase != test.phrase || got.Category != test.category || got.Severity != test.severity {
				t.Fatalf("Find(%q) = %+v", test.text, got)
			}
			if test.text[got.Start:got.End] != test.phrase {
				t.Fatalf("span %d:%d = %q", got.Start, got.End, test.text[got.Start:got.End])
			}
		})
	}
}

func TestFindCharacterInspectionSupportsConsecutiveSentences(t *testing.T) {
	text := "His right arm is badly scratched. His right shoulder has a large bruise. His right foot has a large bruise."
	got := Find(text)
	want := []Match{
		{Start: 17, End: 32, Phrase: "badly scratched", Category: Cuts, Severity: 2},
		{Start: 59, End: 71, Phrase: "large bruise", Category: Bruises, Severity: 2},
		{Start: 94, End: 106, Phrase: "large bruise", Category: Bruises, Severity: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Find(%q) = %#v, want %#v", text, got, want)
	}
}

func TestFindRejectsInspectionWordsOutsideCharacterWoundGrammar(t *testing.T) {
	tests := []string{
		"The tabletop is badly scratched.",
		"This right arm is badly scratched.",
		"Her armor is pierced and her cloak is slashed.",
		"His right arm has a large bruise-colored tattoo.",
		"Her face is completely black and blue beneath the painted mask.",
	}
	for _, text := range tests {
		if got := Find(text); len(got) != 0 {
			t.Errorf("Find(%q) = %+v, want no matches", text, got)
		}
	}
}

func TestFindIsCaseInsensitiveAndUsesLongestPhrase(t *testing.T) {
	text := "He has a DEEP CUT and a cut."
	got := Find(text)
	want := []Match{
		{Start: 9, End: 17, Phrase: "DEEP CUT", Category: Cuts, Severity: 3},
		{Start: 24, End: 27, Phrase: "cut", Category: Cuts, Severity: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Find(%q) = %#v, want %#v", text, got, want)
	}
}

func TestFindRejectsAmbiguousAndUnverifiedText(t *testing.T) {
	tests := []string{
		"Cut the rope before the fire can burn it.",
		"The tailor made deep cuts in the cloth.",
		"She has treated cuts and healing bruises.",
		"He has a ragged wound on his shoulder.",
		"She has a catastrophic burn on her hand.",
		"The word scutwork is not an injury.",
	}
	for _, text := range tests {
		if got := Find(text); len(got) != 0 {
			t.Errorf("Find(%q) = %+v, want no matches", text, got)
		}
	}
}

func TestApplyPreservesTextAndSegmentBoundaries(t *testing.T) {
	segments := []types.StyledSegment{
		{Text: "She has a deep ", Color: "#111111", Italic: true},
		{Text: "cut on her arm and a severe", Underline: true},
		{Text: " burn.", Color: "#222222"},
	}
	cfg := config.WoundHighlightConfig{
		Enabled:   true,
		Cuts:      "red",
		Bruises:   "purple",
		Punctures: "blue",
		Burns:     "orange",
	}

	got := Apply(segments, cfg)
	var text string
	for _, seg := range got {
		text += seg.Text
	}
	if text != "She has a deep cut on her arm and a severe burn." {
		t.Fatalf("Apply changed text: %q", text)
	}

	cutColor, _ := FamilyColor("red", 3)
	burnColor, _ := FamilyColor("orange", 4)
	assertStyledPhrase(t, got, "deep cut", cutColor, false)
	assertStyledPhrase(t, got, "severe burn", burnColor, true)

	// Styling inherited from the source segment survives splitting.
	if !got[0].Italic || got[0].Text != "She has a " {
		t.Fatalf("first source style/boundary not preserved: %+v", got)
	}
}

func TestApplyDisabledReturnsInputUnchanged(t *testing.T) {
	segments := []types.StyledSegment{{Text: "She has a fracture on her arm.", Color: "#abcdef"}}
	got := Apply(segments, config.WoundHighlightConfig{})
	if !reflect.DeepEqual(got, segments) {
		t.Fatalf("disabled Apply = %+v, want %+v", got, segments)
	}
}

func TestApplyHandlesUnicodeBeforeCrossSegmentPhrase(t *testing.T) {
	segments := []types.StyledSegment{{Text: "Éowyn has a gaping "}, {Text: "wound on her hand."}}
	cfg := config.WoundHighlightConfig{Enabled: true, Cuts: "red", Bruises: "purple", Punctures: "blue", Burns: "orange"}
	got := Apply(segments, cfg)
	color, _ := FamilyColor("blue", 5)
	assertStyledPhrase(t, got, "gaping wound", color, true)
}

func assertStyledPhrase(t *testing.T, segments []types.StyledSegment, phrase, color string, bold bool) {
	t.Helper()
	var joined string
	for _, seg := range segments {
		if seg.Color == color && seg.Bold == bold {
			joined += seg.Text
		}
	}
	if joined != phrase {
		t.Fatalf("styled text for %q = %q in %+v", phrase, joined, segments)
	}
}
