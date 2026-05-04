package main

import "strings"

type kudosCommandKind int

const (
	kudosNotKudos kudosCommandKind = iota
	kudosOpenMenu
	kudosAddFavorite
	kudosAddQueue
)

// isKudosCommand reports whether input begins with "/kudos" (case-insensitive,
// followed by EOF or whitespace). The caller is expected to pass a string with
// any surrounding whitespace already trimmed.
func isKudosCommand(input string) bool {
	const prefix = "/kudos"
	if len(input) < len(prefix) {
		return false
	}
	if !strings.EqualFold(input[:len(prefix)], prefix) {
		return false
	}
	if len(input) == len(prefix) {
		return true
	}
	return input[len(prefix)] == ' ' || input[len(prefix)] == '\t'
}

// parseKudosCommand interprets a kudos slash command. Callers must verify
// isKudosCommand(input) is true beforehand. Returns the command kind and
// trimmed name + message (each may be empty depending on kind).
func parseKudosCommand(input string) (kudosCommandKind, string, string) {
	const prefix = "/kudos"
	rest := strings.TrimSpace(input[len(prefix):])
	if rest == "" {
		return kudosOpenMenu, "", ""
	}
	idx := strings.IndexAny(rest, " \t")
	if idx < 0 {
		return kudosAddFavorite, rest, ""
	}
	name := rest[:idx]
	msg := strings.TrimSpace(rest[idx+1:])
	if msg == "" {
		return kudosAddFavorite, name, ""
	}
	return kudosAddQueue, name, msg
}

// shouldShowKudosPrompt returns true when the once-per-process kudos
// login prompt should fire: the prompt has not yet been shown, the
// queue has at least one entry, and the first SKOOT map data (rooms)
// has arrived in this batch.
func shouldShowKudosPrompt(promptShown bool, queueLen, roomCount int) bool {
	return !promptShown && queueLen > 0 && roomCount > 0
}
