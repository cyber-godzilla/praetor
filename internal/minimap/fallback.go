package minimap

import (
	"runtime"
	"strings"
)

// fallbackPlaceholder returns a text-only block that fills the minimap's
// reserved space when no graphics protocol is available. The block is
// exactly `rows` lines tall and `width` columns wide.
func fallbackPlaceholder(width, rows int) string {
	if width < 20 || rows < 5 {
		return shortFallback(width, rows)
	}
	return boxedFallback(width, rows)
}

func shortFallback(width, rows int) string {
	msg := "Minimap unavailable"
	if len(msg) > width {
		msg = msg[:width]
	}
	padded := msg + strings.Repeat(" ", max(0, width-len(msg)))
	lines := make([]string, rows)
	mid := rows / 2
	for i := range lines {
		if i == mid {
			lines[i] = padded
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
	}
	return strings.Join(lines, "\n")
}

func boxedFallback(width, rows int) string {
	inner := width - 2
	body := []string{
		"",
		"Minimap unavailable",
		"",
		"Your terminal doesn't support",
		"Kitty or Sixel graphics.",
		"",
		"Try: " + recommendation(),
		"",
	}

	lines := make([]string, 0, rows)
	lines = append(lines, "┌"+strings.Repeat("─", inner)+"┐")

	bodyRows := rows - 2
	bodyOffset := (bodyRows - len(body)) / 2
	if bodyOffset < 0 {
		bodyOffset = 0
	}

	for i := 0; i < bodyRows; i++ {
		idx := i - bodyOffset
		var text string
		if idx >= 0 && idx < len(body) {
			text = body[idx]
		}
		if len(text) > inner {
			text = text[:inner]
		}
		pad := inner - len(text)
		left := pad / 2
		right := pad - left
		lines = append(lines, "│"+strings.Repeat(" ", left)+text+strings.Repeat(" ", right)+"│")
	}

	lines = append(lines, "└"+strings.Repeat("─", inner)+"┘")
	return strings.Join(lines, "\n")
}

func recommendation() string {
	switch runtime.GOOS {
	case "darwin":
		return "WezTerm, Kitty, iTerm2, or Ghostty"
	case "linux":
		return "WezTerm, Kitty, foot, or Ghostty"
	case "windows":
		return "WezTerm or Windows Terminal"
	}
	return "WezTerm, Kitty, or Ghostty"
}
