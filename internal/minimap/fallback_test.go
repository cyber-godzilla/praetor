package minimap

import (
	"strings"
	"testing"
)

func TestFallbackPlaceholder_ContainsUnavailableText(t *testing.T) {
	out := fallbackPlaceholder(38, 12)
	if !strings.Contains(out, "Minimap unavailable") {
		t.Errorf("expected placeholder to contain 'Minimap unavailable', got:\n%s", out)
	}
}

func TestFallbackPlaceholder_ContainsRecommendation(t *testing.T) {
	out := fallbackPlaceholder(38, 12)
	candidates := []string{"WezTerm", "Kitty", "Ghostty", "iTerm2", "foot", "Windows Terminal"}
	var hit bool
	for _, c := range candidates {
		if strings.Contains(out, c) {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("expected placeholder to contain a terminal recommendation, got:\n%s", out)
	}
}

func TestFallbackPlaceholder_RowCount(t *testing.T) {
	out := fallbackPlaceholder(38, 12)
	rows := strings.Count(out, "\n") + 1
	if rows != 12 {
		t.Errorf("expected 12 rows, got %d\n%s", rows, out)
	}
}

func TestFallbackPlaceholder_NarrowFallsBackToShortForm(t *testing.T) {
	out := fallbackPlaceholder(10, 12)
	if !strings.Contains(out, "Minimap unavailable") {
		t.Errorf("short-form placeholder should still mention minimap, got:\n%s", out)
	}
}
