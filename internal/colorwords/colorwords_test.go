package colorwords

import (
	"testing"

	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestColorWords_HyphenatedColor(t *testing.T) {
	seg := types.StyledSegment{Text: "A loose sheer blue-white shirt"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "blue-white" && s.Color == "#aabbdd" {
			found = true
		}
	}
	if !found {
		t.Errorf("blue-white not colored, got: %+v", result)
	}
}

func TestColorWords_MultiWordWithAdjective(t *testing.T) {
	seg := types.StyledSegment{Text: "Some shimmering deep red leather boots"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "shimmering deep red" && s.Color == "#990000" {
			found = true
		}
	}
	if !found {
		t.Errorf("shimmering deep red not matched, got: %+v", result)
	}
}

func TestColorWords_RustyOrange(t *testing.T) {
	seg := types.StyledSegment{Text: "A rusty orange small wooden carving"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "rusty orange" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("rusty orange not matched, got: %+v", result)
	}
}

func TestColorWords_SimpleColors(t *testing.T) {
	tests := []struct {
		text  string
		word  string
		color string
	}{
		{"A copper keyring", "copper", "#b87333"},
		{"A silver lockpick", "silver", "#aaaaaa"},
		{"An iron cuirass", "iron", "#888899"},
	}

	for _, tc := range tests {
		seg := types.StyledSegment{Text: tc.text}
		result := splitColorWords(seg)

		found := false
		for _, s := range result {
			if s.Text == tc.word && s.Color == tc.color {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: %q not colored %s, got: %+v", tc.text, tc.word, tc.color, result)
		}
	}
}

func TestColorWords_OceanBlue(t *testing.T) {
	seg := types.StyledSegment{Text: "An ocean blue cloak with burnished ochre trim"}
	result := splitColorWords(seg)

	foundOcean := false
	foundOchre := false
	for _, s := range result {
		if s.Text == "ocean blue" && s.Color == "#2266aa" {
			foundOcean = true
		}
		if s.Text == "burnished ochre" && s.Color != "" {
			foundOchre = true
		}
	}
	if !foundOcean {
		t.Errorf("ocean blue not matched, got: %+v", result)
	}
	if !foundOchre {
		t.Errorf("burnished ochre not matched, got: %+v", result)
	}
}

func TestColorWords_PrecedenceOverGameColor(t *testing.T) {
	seg := types.StyledSegment{Text: "A scarlet cloak", Color: "#aaaaaa"}
	result := ApplyColorWords([]types.StyledSegment{seg})

	found := false
	for _, s := range result {
		if s.Text == "scarlet" && s.Color == "#cc2200" {
			found = true
		}
	}
	if !found {
		t.Errorf("scarlet should override game color, got: %+v", result)
	}
}

func TestColorWords_DeepBlack(t *testing.T) {
	seg := types.StyledSegment{Text: "Some deep black leathery gloves"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "deep black" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("deep black not matched, got: %+v", result)
	}
}

func TestColorWords_DiaphanousSnowWhite(t *testing.T) {
	seg := types.StyledSegment{Text: "diaphanous pale blue silk"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "diaphanous pale blue" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("diaphanous pale blue not matched, got: %+v", result)
	}
}

func TestColorWords_NoPartialMatch(t *testing.T) {
	seg := types.StyledSegment{Text: "The goldenrods flower blooms"}
	result := splitColorWords(seg)

	for _, s := range result {
		if s.Text == "golden" && s.Color != "" {
			t.Errorf("golden should not match inside goldenrods, got: %+v", result)
		}
	}
}

func TestColorWords_GoldenrodAsColor(t *testing.T) {
	seg := types.StyledSegment{Text: "A length of goldenrod doeskin cloth"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "goldenrod" && s.Color == "#ccaa22" {
			found = true
		}
	}
	if !found {
		t.Errorf("goldenrod not matched, got: %+v", result)
	}
}

func TestColorWords_RustColored(t *testing.T) {
	seg := types.StyledSegment{Text: "A spool of rust-colored thread"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "rust-colored" && s.Color == "#b7410e" {
			found = true
		}
	}
	if !found {
		t.Errorf("rust-colored not matched, got: %+v", result)
	}
}

func TestColorWords_Sinopia(t *testing.T) {
	seg := types.StyledSegment{Text: "A spool of sinopia thread"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "sinopia" && s.Color == "#993311" {
			found = true
		}
	}
	if !found {
		t.Errorf("sinopia not matched, got: %+v", result)
	}
}

func TestColorWords_MidnightBlue(t *testing.T) {
	seg := types.StyledSegment{Text: "A midnight blue cloak"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "midnight blue" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("midnight blue not matched, got: %+v", result)
	}
}

func TestColorWords_SlateBlueSingleColor(t *testing.T) {
	seg := types.StyledSegment{Text: "A slate blue shirt"}
	result := splitColorWords(seg)

	for _, s := range result {
		if s.Text == "slate" && s.Color != "" {
			t.Errorf("slate should not match separately when slate blue is a phrase, got: %+v", result)
		}
	}
	found := false
	for _, s := range result {
		if s.Text == "slate blue" && s.Color == "#5566aa" {
			found = true
		}
	}
	if !found {
		t.Errorf("slate blue not matched as single color, got: %+v", result)
	}
}

func TestColorWords_MottledAdjective(t *testing.T) {
	seg := types.StyledSegment{Text: "A mottled green cloak"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "mottled green" && s.Color == "#00aa00" {
			found = true
		}
	}
	if !found {
		t.Errorf("mottled green not matched, got: %+v", result)
	}
}

func TestColorWords_DuskyAdjective(t *testing.T) {
	seg := types.StyledSegment{Text: "A dusky red shirt"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "dusky red" && s.Color == "#cc0000" {
			found = true
		}
	}
	if !found {
		t.Errorf("dusky red not matched, got: %+v", result)
	}
}

func TestColorWords_PluralAlternation(t *testing.T) {
	seg := types.StyledSegment{Text: "The reds and blues clash"}
	result := splitColorWords(seg)

	// Plurals should produce per-character segments with alternating shades.
	// Find the 'r' and 'e' of "reds" — they should have different colors.
	var rColor, eColor string
	for i, s := range result {
		if s.Text == "r" && s.Color != "" && i+1 < len(result) && result[i+1].Text == "e" {
			rColor = s.Color
			eColor = result[i+1].Color
			break
		}
	}
	if rColor == "" {
		t.Error("reds not colored")
	}
	if rColor == eColor {
		t.Errorf("reds should alternate colors, got %s and %s", rColor, eColor)
	}
}

func TestColorWords_ForestAsAdjective(t *testing.T) {
	seg := types.StyledSegment{Text: "A forest path"}
	result := splitColorWords(seg)
	for _, s := range result {
		if s.Text == "forest" && s.Color != "" {
			t.Errorf("forest alone should not be colored, got: %+v", result)
		}
	}

	seg2 := types.StyledSegment{Text: "A forest green cloak"}
	result2 := splitColorWords(seg2)
	found := false
	for _, s := range result2 {
		if s.Text == "forest green" && s.Color == "#226622" {
			found = true
		}
	}
	if !found {
		t.Errorf("forest green not matched, got: %+v", result2)
	}
}

func TestColorWords_MotherOfPearl(t *testing.T) {
	seg := types.StyledSegment{Text: "A mother of pearl button"}
	result := splitColorWords(seg)
	found := false
	for _, s := range result {
		if s.Text == "mother of pearl" && s.Color == "#ccccdd" {
			found = true
		}
	}
	if !found {
		t.Errorf("mother of pearl not matched, got: %+v", result)
	}
}

func TestColorWords_BurntOrange(t *testing.T) {
	seg := types.StyledSegment{Text: "A burnt orange scarf"}
	result := splitColorWords(seg)
	found := false
	for _, s := range result {
		if s.Text == "burnt orange" && s.Color == "#cc5500" {
			found = true
		}
	}
	if !found {
		t.Errorf("burnt orange not matched, got: %+v", result)
	}
}

func TestColorWords_DarkFernGreen(t *testing.T) {
	seg := types.StyledSegment{Text: "A dark fern green tunic"}
	result := splitColorWords(seg)
	found := false
	for _, s := range result {
		if s.Text == "dark fern green" && s.Color == "#447733" {
			found = true
		}
	}
	if !found {
		t.Errorf("dark fern green not matched, got: %+v", result)
	}
}

func TestColorWords_Blond(t *testing.T) {
	seg := types.StyledSegment{Text: "A blond leather belt"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "blond" && s.Color == "#ddcc77" {
			found = true
		}
	}
	if !found {
		t.Errorf("blond not matched, got: %+v", result)
	}
}

func TestColorWords_CharacterDescriptions(t *testing.T) {
	tests := []struct {
		text     string
		expected string
		color    string
	}{
		{"sallow skin", "sallow", "#ccbb77"},
		{"coal-black hair", "coal-black", "#505050"},
		{"honey blonde hair", "honey blonde", "#ddbb55"},
		{"strawberry blonde curls", "strawberry blonde", "#cc9966"},
		{"ash brown hair", "ash brown", "#887766"},
		{"roan hair", "roan", "#884444"},
		{"ginger hair", "ginger", "#cc6622"},
		{"milky white eyes", "milky white", "#cccccc"},
		{"jade green eyes", "jade green", "#00aa66"},
		{"sea green eyes", "sea green", "#339977"},
		{"swarthy complexion", "swarthy", "#664433"},
	}

	for _, tc := range tests {
		seg := types.StyledSegment{Text: tc.text}
		result := splitColorWords(seg)

		found := false
		for _, s := range result {
			if s.Text == tc.expected && s.Color == tc.color {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: expected %q in %s, got: %+v", tc.text, tc.expected, tc.color, result)
		}
	}
}

func TestColorWords_SuffixTipped(t *testing.T) {
	seg := types.StyledSegment{Text: "A bronze-tipped spear"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "bronze-tipped" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("bronze-tipped not matched, got: %+v", result)
	}
}

func TestColorWords_SuffixStained(t *testing.T) {
	seg := types.StyledSegment{Text: "A blood-stained cloth"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		// "blood" isn't a color word, but "red-stained" would be.
		// Let's test with a real color.
		_ = s
	}

	seg2 := types.StyledSegment{Text: "A crimson-stained blade"}
	result2 := splitColorWords(seg2)
	found = false
	for _, s := range result2 {
		if s.Text == "crimson-stained" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("crimson-stained not matched, got: %+v", result2)
	}
}

func TestColorWords_SuffixLined(t *testing.T) {
	seg := types.StyledSegment{Text: "A silver-lined cloak"}
	result := splitColorWords(seg)

	found := false
	for _, s := range result {
		if s.Text == "silver-lined" && s.Color != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("silver-lined not matched, got: %+v", result)
	}
}

func TestColorWords_Bloodstained(t *testing.T) {
	for _, text := range []string{"A bloodstained rag", "A blood-stained cloth"} {
		seg := types.StyledSegment{Text: text}
		result := splitColorWords(seg)

		found := false
		for _, s := range result {
			if (s.Text == "bloodstained" || s.Text == "blood-stained") && s.Color != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: bloodstained not matched, got: %+v", text, result)
		}
	}
}

func TestColorWords_Rainbow(t *testing.T) {
	seg := types.StyledSegment{Text: "A rainbow scarf"}
	result := splitColorWords(seg)

	// Rainbow should produce per-character segments.
	colorCount := 0
	for _, s := range result {
		if len(s.Text) == 1 && s.Color != "" {
			colorCount++
		}
	}
	if colorCount < 5 {
		t.Errorf("rainbow should produce per-character colored segments, got %d colored chars in: %+v", colorCount, result)
	}
}

func TestColorWords_Multicolored(t *testing.T) {
	seg := types.StyledSegment{Text: "A multicolored gem"}
	result := splitColorWords(seg)

	colorCount := 0
	for _, s := range result {
		if len(s.Text) == 1 && s.Color != "" {
			colorCount++
		}
	}
	if colorCount < 5 {
		t.Errorf("multicolored should produce per-character colored segments, got %d colored chars in: %+v", colorCount, result)
	}
}
