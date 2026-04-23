package graphics

import (
	"os"
	"strings"
	"sync"
)

var (
	detectOnce sync.Once
	detected   Mode
)

// Detect inspects the process environment once and returns the inferred
// terminal graphics mode. The result is memoised for the life of the
// process.
func Detect() Mode {
	detectOnce.Do(func() {
		detected = detectFromEnv(os.Getenv)
	})
	return detected
}

// detectFromEnv implements the detection rules against an arbitrary
// environment lookup. Separated from Detect so tests can feed synthetic
// environments without mutating the real process env.
func detectFromEnv(get func(string) string) Mode {
	switch strings.ToLower(get("PRAETOR_GRAPHICS")) {
	case "kitty":
		return ModeKitty
	case "sixel":
		return ModeSixel
	case "none":
		return ModeNone
	}

	termProgram := strings.ToLower(get("TERM_PROGRAM"))
	switch termProgram {
	case "ghostty", "wezterm", "iterm.app":
		return ModeKitty
	}

	if get("KITTY_WINDOW_ID") != "" {
		return ModeKitty
	}
	if get("KONSOLE_VERSION") != "" {
		return ModeKitty
	}
	if get("TERM") == "xterm-kitty" {
		return ModeKitty
	}

	if get("WT_SESSION") != "" {
		return ModeSixel
	}
	if termProgram == "mintty" {
		return ModeSixel
	}
	term := get("TERM")
	if strings.Contains(term, "mlterm") || strings.Contains(term, "foot") {
		return ModeSixel
	}

	return ModeNone
}
