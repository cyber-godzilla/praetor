// Package speechcolor recognizes character speech lines and applies a configured
// Colorword style to the entire rendered line.
package speechcolor

import (
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/commandinput"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

var characterPatternCache sync.Map // map[string]*regexp.Regexp

var speechVerbs = map[string]bool{
	"adds": true, "admits": true, "announces": true, "answers": true,
	"asks": true, "begs": true, "breathes": true, "calls": true,
	"chuckles": true, "continues": true, "cries": true, "croaks": true,
	"declares": true, "demands": true, "exclaims": true, "gasps": true,
	"growls": true, "hisses": true, "insists": true, "intones": true,
	"laughs": true, "murmurs": true, "mutters": true, "pleads": true,
	"remarks": true, "replies": true, "responds": true, "says": true,
	"shouts": true, "sighs": true, "snaps": true, "speaks": true,
	"stammers": true, "stutters": true, "tells": true, "warns": true,
	"whispers": true, "yells": true,
}

type lineKind uint8

const (
	kindSpeech lineKind = iota
	kindThink
	kindOOC
)

// Apply returns segments colored by the first active configured character
// pattern whose name and speech grammar match text. Character patterns expand
// input variables and support case-insensitive * and ? wildcards across the
// complete speaker name. Non-speech lines are returned unchanged.
func Apply(segments []types.StyledSegment, text string, rules []config.SpeechColorConfig, variables map[string]string) []types.StyledSegment {
	speaker, kind, ok := speakerAndKind(text)
	if !ok {
		return segments
	}
	for _, rule := range rules {
		if !rule.Active || !matchesCharacter(rule.Character, speaker, variables) {
			continue
		}
		if kind == kindThink && !rule.IncludeThink {
			continue
		}
		if kind == kindOOC && !rule.IncludeOOC {
			continue
		}
		return colorwords.ApplyStyle(segments, rule.Style)
	}
	return segments
}

func matchesCharacter(pattern, speaker string, variables map[string]string) bool {
	expanded, err := commandinput.ExpandVariables(strings.TrimSpace(pattern), variables)
	if err != nil || expanded == "" {
		return false
	}
	if !strings.ContainsAny(expanded, "*?") {
		return strings.EqualFold(expanded, speaker)
	}
	if cached, ok := characterPatternCache.Load(expanded); ok {
		return cached.(*regexp.Regexp).MatchString(speaker)
	}
	escaped := regexp.QuoteMeta(expanded)
	escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
	escaped = strings.ReplaceAll(escaped, `\?`, `.`)
	compiled := regexp.MustCompile(`(?i)^` + escaped + `\z`)
	actual, _ := characterPatternCache.LoadOrStore(expanded, compiled)
	return actual.(*regexp.Regexp).MatchString(speaker)
}

// Speaker returns the character/account name at the start of a speech line.
// It recognizes ordinary speech, OOC channel prefixes, and think-aloud lines.
// Ordinary speech must contain the game's comma-and-quoted-text form so prose
// such as "The sign says danger" is not offered as character speech.
func Speaker(text string) (string, bool) {
	speaker, _, ok := speakerAndKind(text)
	return speaker, ok
}

func speakerAndKind(text string) (string, lineKind, bool) {
	line := strings.TrimSpace(text)
	if line == "" {
		return "", kindSpeech, false
	}

	// Think-aloud lines begin with an opening angle bracket whose closing bracket
	// comes after the speech, unlike the timestamp/channel prefix used by OOC.
	if strings.HasPrefix(line, "<") {
		if marker := strings.Index(strings.ToLower(line), " thinks aloud:"); marker > 1 {
			name := strings.TrimSpace(line[1:marker])
			if name != "" {
				return name, kindThink, true
			}
		}
		// Channel lines have a closed prefix such as "<8:14 pm OOC>" before the
		// account/character name. Treat them like ordinary speech after the prefix.
		if close := strings.IndexByte(line, '>'); close >= 0 {
			fields := strings.Fields(line[1:close])
			kind := kindSpeech
			if len(fields) > 0 && strings.EqualFold(fields[len(fields)-1], "OOC") {
				kind = kindOOC
			}
			line = strings.TrimSpace(line[close+1:])
			return ordinarySpeaker(line, kind)
		}
	}
	return ordinarySpeaker(line, kindSpeech)
}

func ordinarySpeaker(line string, kind lineKind) (string, lineKind, bool) {
	type word struct {
		start int
		end   int
		value string
	}
	var words []word
	for offset := 0; offset < len(line); {
		r, size := utf8.DecodeRuneInString(line[offset:])
		if !unicode.IsLetter(r) {
			offset += size
			continue
		}
		start := offset
		for offset < len(line) {
			r, size = utf8.DecodeRuneInString(line[offset:])
			if !unicode.IsLetter(r) && r != '\'' && r != '-' {
				break
			}
			offset += size
		}
		words = append(words, word{start: start, end: offset, value: strings.ToLower(line[start:offset])})
	}
	for index, candidate := range words {
		if !speechVerbs[candidate.value] || !hasQuotedSpeech(line[candidate.end:]) {
			continue
		}
		nameEnd := candidate.start
		// TEC may place one or more delivery adverbs between the character and
		// verb ("Apula very softly whispers"). They are not part of the name.
		for index > 0 && strings.HasSuffix(words[index-1].value, "ly") {
			nameEnd = words[index-1].start
			index--
		}
		name := strings.TrimSpace(line[:nameEnd])
		if name != "" {
			return name, kind, true
		}
	}
	return "", kind, false
}

func hasQuotedSpeech(afterVerb string) bool {
	afterVerb = strings.TrimSpace(afterVerb)
	if !strings.HasPrefix(afterVerb, ",") {
		return false
	}
	afterVerb = strings.TrimSpace(strings.TrimPrefix(afterVerb, ","))
	if afterVerb == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(afterVerb)
	return first == '"' || first == '\'' || first == '“'
}
