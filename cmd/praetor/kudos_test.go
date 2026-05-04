package main

import (
	"testing"
)

func TestParseKudosCommand(t *testing.T) {
	cases := []struct {
		input    string
		wantKind kudosCommandKind
		wantName string
		wantMsg  string
	}{
		{"/kudos", kudosOpenMenu, "", ""},
		{"/kudos ", kudosOpenMenu, "", ""},
		{"/kudos    ", kudosOpenMenu, "", ""},
		{"/kudos Alice", kudosAddFavorite, "Alice", ""},
		{"/kudos  Alice  ", kudosAddFavorite, "Alice", ""},
		{"/kudos Alice you saved my life", kudosAddQueue, "Alice", "you saved my life"},
		{"/kudos Bob   thanks for the rescue  ", kudosAddQueue, "Bob", "thanks for the rescue"},
		{"/kudos Alice   ", kudosAddFavorite, "Alice", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			kind, name, msg := parseKudosCommand(tc.input)
			if kind != tc.wantKind {
				t.Errorf("kind=%v want %v", kind, tc.wantKind)
			}
			if name != tc.wantName {
				t.Errorf("name=%q want %q", name, tc.wantName)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg=%q want %q", msg, tc.wantMsg)
			}
		})
	}
}

func TestIsKudosCommand(t *testing.T) {
	cases := map[string]bool{
		"/kudos":             true,
		"/kudos ":            true,
		"/kudos Alice":       true,
		"/kudos Alice text":  true,
		"/kudo":              false,
		"/kudosa":            false,
		"/Kudos":             true,
		"/KUDOS Alice":       true,
		"":                   false,
		"kudos Alice":        false,
		" /kudos Alice":      false,
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			if got := isKudosCommand(input); got != want {
				t.Errorf("isKudosCommand(%q)=%v want %v", input, got, want)
			}
		})
	}
}

func TestShouldShowKudosPrompt(t *testing.T) {
	cases := []struct {
		shown    bool
		queueLen int
		rooms    int
		want     bool
	}{
		{false, 1, 1, true},
		{false, 0, 1, false},
		{false, 5, 0, false},
		{true, 5, 1, false},
	}
	for _, tc := range cases {
		got := shouldShowKudosPrompt(tc.shown, tc.queueLen, tc.rooms)
		if got != tc.want {
			t.Errorf("shouldShowKudosPrompt(shown=%v, q=%d, rooms=%d)=%v want %v",
				tc.shown, tc.queueLen, tc.rooms, got, tc.want)
		}
	}
}
