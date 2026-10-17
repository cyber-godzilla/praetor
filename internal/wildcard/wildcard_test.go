package wildcard

import (
	"encoding/json"
	"os"
	"testing"
)

type conformanceCase struct {
	Name    string   `json:"name"`
	Pattern string   `json:"pattern"`
	Text    string   `json:"text"`
	Scope   string   `json:"scope"`
	Match   bool     `json:"match"`
	Spans   []string `json:"spans"`
}

func loadConformance(t *testing.T) []conformanceCase {
	t.Helper()
	data, err := os.ReadFile("testdata/conformance.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []conformanceCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestConformance(t *testing.T) {
	for _, test := range loadConformance(t) {
		t.Run(test.Name, func(t *testing.T) {
			scope := Substring
			if test.Scope == "whole" {
				scope = Whole
			}
			pattern, err := Compile(test.Pattern, scope)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got := pattern.Match(test.Text); got != test.Match {
				t.Errorf("Match = %v, want %v", got, test.Match)
			}
			spans := pattern.FindAll(test.Text)
			got := make([]string, 0, len(spans))
			for _, span := range spans {
				got = append(got, test.Text[span.Start:span.End])
			}
			if len(got) != len(test.Spans) {
				t.Fatalf("FindAll = %q, want %q", got, test.Spans)
			}
			for i := range got {
				if got[i] != test.Spans[i] {
					t.Errorf("FindAll[%d] = %q, want %q", i, got[i], test.Spans[i])
				}
			}
		})
	}
}
