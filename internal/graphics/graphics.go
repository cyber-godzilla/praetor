// Package graphics owns terminal graphics capability detection and encoder
// dispatch for praetor. Callers obtain a Mode via Detect(), then pass that
// Mode (plus an image) through Encode to get the right escape sequence.
package graphics

import (
	"image"

	"github.com/cyber-godzilla/praetor/internal/kitty"
	"github.com/cyber-godzilla/praetor/internal/sixel"
)

// Mode identifies the active terminal graphics protocol.
type Mode int

const (
	ModeNone Mode = iota
	ModeSixel
	ModeKitty
)

func (m Mode) String() string {
	switch m {
	case ModeKitty:
		return "kitty"
	case ModeSixel:
		return "sixel"
	default:
		return "none"
	}
}

// Encode dispatches img to the encoder for the given Mode. Returns "" when
// mode is ModeNone. cols and rows are the terminal cell dimensions for
// display (Kitty uses them; Sixel ignores them).
func Encode(mode Mode, img *image.RGBA, cols, rows int) string {
	switch mode {
	case ModeKitty:
		return kitty.Encode(img, cols, rows)
	case ModeSixel:
		return sixel.Encode(img, cols, rows)
	}
	return ""
}
