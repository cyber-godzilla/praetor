package speechcolor

import (
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestApplyRecognizesSpeechFormsAndColorsWholeLine(t *testing.T) {
	rules := []config.SpeechColorConfig{{Character: "Apula", Style: "cinnabar", Active: true}}
	for _, text := range []string{
		`Apula says, "Hello."`,
		`Apula softly whispers, "Not here."`,
	} {
		out := Apply([]types.StyledSegment{{Text: text}}, text, rules, nil)
		if len(out) != 1 || out[0].Color != "#cc3322" {
			t.Errorf("Apply(%q) = %+v, want whole cinnabar line", text, out)
		}
	}
}

func TestApplyMatchesCharacterWildcardsAndVariables(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		speaker   string
		variables map[string]string
		wantColor bool
	}{
		{name: "star", pattern: "A * cloak", speaker: "A man in a hooded cloak", wantColor: true},
		{name: "question mark", pattern: "Apul?", speaker: "Apula", wantColor: true},
		{name: "case insensitive", pattern: "ap*", speaker: "Apula", wantColor: true},
		{name: "whole name only", pattern: "pula", speaker: "Apula", wantColor: false},
		{name: "variable", pattern: "${speaker}", speaker: "Constantine", variables: map[string]string{"speaker": "Constantine"}, wantColor: true},
		{name: "variable containing wildcard", pattern: "${speaker}", speaker: "A man in a hooded cloak", variables: map[string]string{"speaker": "A * cloak"}, wantColor: true},
		{name: "fallback", pattern: "${speaker:Apula}", speaker: "Apula", wantColor: true},
		{name: "unknown variable", pattern: "${speaker}", speaker: "Apula", wantColor: false},
		{name: "malformed variable", pattern: "${speaker", speaker: "Apula", wantColor: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			text := test.speaker + ` says, "Hello."`
			rules := []config.SpeechColorConfig{{Character: test.pattern, Style: "cinnabar", Active: true}}
			out := Apply([]types.StyledSegment{{Text: text}}, text, rules, test.variables)
			if got := out[0].Color != ""; got != test.wantColor {
				t.Fatalf("colored = %v, want %v; output %+v", got, test.wantColor, out)
			}
		})
	}
}

func TestApplyThinkAndOOCAreIndependentlyOptional(t *testing.T) {
	think := `<Apula thinks aloud: I wonder...>`
	ooc := `<8:14 pm OOC> Apula replies, "Hello."`
	plain := []types.StyledSegment{{Text: think}}

	base := config.SpeechColorConfig{Character: "Apula", Style: "cinnabar", Active: true}
	if out := Apply(plain, think, []config.SpeechColorConfig{base}, nil); out[0].Color != "" {
		t.Fatalf("think line colored without opt-in: %+v", out)
	}
	if out := Apply([]types.StyledSegment{{Text: ooc}}, ooc, []config.SpeechColorConfig{base}, nil); out[0].Color != "" {
		t.Fatalf("OOC line colored without opt-in: %+v", out)
	}

	base.IncludeThink = true
	if out := Apply(plain, think, []config.SpeechColorConfig{base}, nil); out[0].Color != "#cc3322" {
		t.Fatalf("think opt-in did not color line: %+v", out)
	}
	if out := Apply([]types.StyledSegment{{Text: ooc}}, ooc, []config.SpeechColorConfig{base}, nil); out[0].Color != "" {
		t.Fatalf("think opt-in also colored OOC line: %+v", out)
	}

	base.IncludeThink = false
	base.IncludeOOC = true
	if out := Apply([]types.StyledSegment{{Text: ooc}}, ooc, []config.SpeechColorConfig{base}, nil); out[0].Color != "#cc3322" {
		t.Fatalf("OOC opt-in did not color line: %+v", out)
	}
	if out := Apply(plain, think, []config.SpeechColorConfig{base}, nil); out[0].Color != "" {
		t.Fatalf("OOC opt-in also colored think line: %+v", out)
	}
}

func TestApplyRejectsOtherCharactersAndNonSpeech(t *testing.T) {
	rules := []config.SpeechColorConfig{{Character: "Apula", Style: "cinnabar", Active: true}}
	for _, text := range []string{
		`Constantine says, "Hello."`,
		`Apula picks up a scroll.`,
		`Apulason says, "Hello."`,
		`The name Apula says nothing about this line.`,
	} {
		input := []types.StyledSegment{{Text: text}}
		out := Apply(input, text, rules, nil)
		if &out[0] != &input[0] || out[0].Color != "" {
			t.Errorf("Apply(%q) unexpectedly changed %+v", text, out)
		}
	}
}

func TestApplySpecialStylePreservesText(t *testing.T) {
	text := `A man in a hooded cloak murmurs, "Go."`
	rules := []config.SpeechColorConfig{{Character: "A man in a hooded cloak", Style: "rainbow", Active: true}}
	out := Apply([]types.StyledSegment{{Text: text, Italic: true}}, text, rules, nil)
	var rebuilt strings.Builder
	colors := map[string]bool{}
	for _, segment := range out {
		rebuilt.WriteString(segment.Text)
		colors[segment.Color] = true
		if !segment.Italic {
			t.Fatalf("special style lost italic attribute: %+v", segment)
		}
	}
	if rebuilt.String() != text || len(colors) < 4 {
		t.Fatalf("rainbow result text/colors = %q/%v", rebuilt.String(), colors)
	}
}

func TestInactiveRuleDoesNotApply(t *testing.T) {
	text := `Apula says, "Hello."`
	rules := []config.SpeechColorConfig{{Character: "Apula", Style: "cinnabar", Active: false}}
	out := Apply([]types.StyledSegment{{Text: text}}, text, rules, nil)
	if out[0].Color != "" {
		t.Fatalf("inactive rule colored line: %+v", out)
	}
}

func TestSpeakerExtractsNamesAndRejectsNarrativeText(t *testing.T) {
	tests := []struct {
		text string
		name string
		ok   bool
	}{
		{`Apula says, "Hello."`, "Apula", true},
		{`Apula softly whispers, "Not here."`, "Apula", true},
		{`A man in a hooded cloak murmurs, "Go."`, "A man in a hooded cloak", true},
		{`<8:14 pm OOC> Constantine replies, "Indeed."`, "Constantine", true},
		{`<Apula thinks aloud: I wonder...>`, "Apula", true},
		{`The sign says danger`, "", false},
		{`Apula picks up a scroll.`, "", false},
	}
	for _, test := range tests {
		name, ok := Speaker(test.text)
		if name != test.name || ok != test.ok {
			t.Errorf("Speaker(%q) = %q, %v; want %q, %v", test.text, name, ok, test.name, test.ok)
		}
	}
}
