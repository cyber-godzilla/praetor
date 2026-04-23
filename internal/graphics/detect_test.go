package graphics

import "testing"

func TestDetectFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Mode
	}{
		{"empty env", nil, ModeNone},
		{"PRAETOR_GRAPHICS=kitty override", map[string]string{"PRAETOR_GRAPHICS": "kitty"}, ModeKitty},
		{"PRAETOR_GRAPHICS=sixel override", map[string]string{"PRAETOR_GRAPHICS": "sixel"}, ModeSixel},
		{"PRAETOR_GRAPHICS=none override", map[string]string{"PRAETOR_GRAPHICS": "none", "TERM_PROGRAM": "ghostty"}, ModeNone},
		{"TERM_PROGRAM=ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, ModeKitty},
		{"TERM_PROGRAM=WezTerm", map[string]string{"TERM_PROGRAM": "WezTerm"}, ModeKitty},
		{"TERM_PROGRAM=iTerm.app", map[string]string{"TERM_PROGRAM": "iTerm.app"}, ModeKitty},
		{"TERM=xterm-kitty", map[string]string{"TERM": "xterm-kitty"}, ModeKitty},
		{"KITTY_WINDOW_ID set", map[string]string{"KITTY_WINDOW_ID": "1"}, ModeKitty},
		{"KONSOLE_VERSION set", map[string]string{"KONSOLE_VERSION": "220000"}, ModeKitty},
		{"WT_SESSION set (Windows Terminal)", map[string]string{"WT_SESSION": "abc-123"}, ModeSixel},
		{"TERM_PROGRAM=mintty", map[string]string{"TERM_PROGRAM": "mintty"}, ModeSixel},
		{"TERM contains foot", map[string]string{"TERM": "foot-extra"}, ModeSixel},
		{"TERM contains mlterm", map[string]string{"TERM": "mlterm"}, ModeSixel},
		{"unknown TERM_PROGRAM", map[string]string{"TERM_PROGRAM": "Terminal.app"}, ModeNone},
		{"empty strings ignored", map[string]string{"TERM_PROGRAM": "", "TERM": ""}, ModeNone},
		{"PRAETOR_GRAPHICS invalid value falls through", map[string]string{"PRAETOR_GRAPHICS": "garbage", "TERM_PROGRAM": "ghostty"}, ModeKitty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			get := func(k string) string {
				if tc.env == nil {
					return ""
				}
				return tc.env[k]
			}
			got := detectFromEnv(get)
			if got != tc.want {
				t.Errorf("detectFromEnv(%v) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}
