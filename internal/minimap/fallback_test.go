package minimap

import (
	"strings"
	"testing"
	"unicode/utf8"
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
	if !strings.Contains(out, "Minimap") {
		t.Errorf("short-form placeholder should still mention Minimap, got:\n%s", out)
	}
}

func TestFallbackPlaceholder_DisplayWidthExact(t *testing.T) {
	const width, rows = 38, 12
	out := fallbackPlaceholder(width, rows)
	for i, line := range strings.Split(out, "\n") {
		got := utf8.RuneCountInString(line)
		if got != width {
			t.Errorf("line %d: got %d display columns, want %d; line=%q", i, got, width, line)
		}
	}
}
