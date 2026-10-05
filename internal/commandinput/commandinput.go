// Package commandinput expands the small, intentionally non-recursive syntax
// supported by Praetor's user-submitted commands and text blocks.
package commandinput

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
)

// MaxCommands limits one typed line so an accidental paste cannot enqueue an
// unbounded burst of commands.
const MaxCommands = 100

// WaitMode controls what must happen before a command after the first one is
// dispatched.
type WaitMode uint8

const (
	WaitNone WaitMode = iota
	WaitDelay
	WaitUnbusy
)

// CommandKind describes what a parsed input step does. KindSend is zero so
// existing callers that construct Command values continue to mean a game or
// local command.
type CommandKind uint8

const (
	KindSend CommandKind = iota
	KindWait
	KindWaitFor
	KindNotify
	KindRepeat
)

// Command is one fully expanded command and the separator that preceded it.
type Command struct {
	Text     string
	Title    string
	Wait     WaitMode
	Kind     CommandKind
	Duration time.Duration
	Timeout  time.Duration
	Match    string
	Cancel   string
	Max      int
}

type part struct {
	text string
	wait WaitMode
}

// Expand splits one typed line on unescaped ;; and && separators and
// substitutes ${name} or ${name:fallback} references in each resulting
// command. ;; waits for the normal pacing delay; && waits for a recognized
// unbusy game line. Splitting happens first, so variable values and fallbacks
// containing separators remain data and cannot inject extra commands.
func Expand(input string, variables map[string]string) ([]Command, error) {
	parts, err := split(input)
	if err != nil {
		return nil, err
	}
	commands := make([]Command, 0, len(parts))
	for _, part := range parts {
		command, err := expandPart(part, variables)
		if err != nil {
			return nil, err
		}
		if len(parts) > 1 {
			command.Text = strings.TrimSpace(command.Text)
			if command.Kind == KindSend && command.Text == "" {
				continue
			}
		}
		wait := part.wait
		if len(commands) == 0 {
			wait = WaitNone
		}
		command.Wait = wait
		commands = append(commands, command)
		if len(commands) > MaxCommands {
			return nil, fmt.Errorf("too many commands (maximum %d)", MaxCommands)
		}
	}
	return commands, nil
}

// split recognizes \;; and \&& as literal double-character sequences. Other
// backslashes are preserved byte-for-byte for the game server. Separators
// inside a $() directive are data, not outer-chain syntax.
func split(input string) ([]part, error) {
	var parts []part
	var text strings.Builder
	wait := WaitNone
	inDirective := false
	inQuote := false
	directiveStart := 0
	for i := 0; i < len(input); i++ {
		if inDirective {
			if input[i] == '\\' && i+1 < len(input) {
				text.WriteByte(input[i])
				text.WriteByte(input[i+1])
				i++
				continue
			}
			if input[i] == '"' {
				inQuote = !inQuote
				text.WriteByte(input[i])
				continue
			}
			if input[i] == ')' && !inQuote {
				inDirective = false
			}
			text.WriteByte(input[i])
			continue
		}
		if input[i] == '\\' && i+2 < len(input) &&
			((input[i+1] == ';' && input[i+2] == ';') || (input[i+1] == '&' && input[i+2] == '&')) {
			text.WriteByte(input[i+1])
			text.WriteByte(input[i+2])
			i += 2
			continue
		}
		if input[i] == '$' && i+1 < len(input) && input[i+1] == '{' {
			end := strings.IndexByte(input[i+2:], '}')
			if end < 0 {
				// Preserve the rest as one atomic reference. ExpandVariables will
				// report an unterminated unescaped reference; an escaped \${...
				// remains literal as before.
				text.WriteString(input[i:])
				break
			}
			end += i + 2
			text.WriteString(input[i : end+1])
			i = end
			continue
		}
		if input[i] == '$' && i+1 < len(input) && input[i+1] == '(' &&
			(i == 0 || input[i-1] != '\\') {
			inDirective = true
			inQuote = false
			directiveStart = i
			text.WriteString("$(")
			i++
			continue
		}
		if input[i] == ';' && i+1 < len(input) && input[i+1] == ';' {
			parts = append(parts, part{text: text.String(), wait: wait})
			text.Reset()
			wait = WaitDelay
			i++
			continue
		}
		if input[i] == '&' && i+1 < len(input) && input[i+1] == '&' {
			parts = append(parts, part{text: text.String(), wait: wait})
			text.Reset()
			wait = WaitUnbusy
			i++
			continue
		}
		text.WriteByte(input[i])
	}
	if inDirective {
		if inQuote {
			return nil, fmt.Errorf("unterminated quoted string in input directive at character %d", directiveStart+1)
		}
		return nil, fmt.Errorf("unterminated input directive at character %d", directiveStart+1)
	}
	parts = append(parts, part{text: text.String(), wait: wait})
	return parts, nil
}

func expandPart(part part, variables map[string]string) (Command, error) {
	raw := strings.TrimSpace(part.text)
	if strings.HasPrefix(raw, "$(") {
		if !strings.HasSuffix(raw, ")") {
			return Command{}, fmt.Errorf("input directive must occupy a complete chain step")
		}
		return expandDirective(raw[2:len(raw)-1], variables)
	}
	if containsUnescapedDirective(raw) {
		return Command{}, fmt.Errorf("input directive must occupy a complete chain step")
	}
	text, err := ExpandVariables(part.text, variables)
	if err != nil {
		return Command{}, err
	}
	return Command{Text: text}, nil
}

func containsUnescapedDirective(input string) bool {
	for i := 0; i+1 < len(input); i++ {
		if input[i] == '$' && input[i+1] == '{' {
			if end := strings.IndexByte(input[i+2:], '}'); end >= 0 {
				i += end + 2
				continue
			}
			return false
		}
		if input[i] == '$' && input[i+1] == '(' && (i == 0 || input[i-1] != '\\') {
			return true
		}
	}
	return false
}

func expandDirective(body string, variables map[string]string) (Command, error) {
	name, arg := splitDirectiveHead(body)
	switch name {
	case "wait":
		expanded, err := ExpandVariables(strings.TrimSpace(arg), variables)
		if err != nil {
			return Command{}, err
		}
		seconds, err := strconv.ParseFloat(expanded, 64)
		if err != nil || seconds <= 0 {
			return Command{}, fmt.Errorf("wait requires a positive number of seconds")
		}
		duration := time.Duration(seconds * float64(time.Second))
		if duration <= 0 {
			return Command{}, fmt.Errorf("wait duration is too small")
		}
		return Command{Kind: KindWait, Duration: duration}, nil

	case "wait-for":
		value, cancel, timeoutText, err := parseWaitForArguments(arg)
		if err != nil {
			return Command{}, err
		}
		value, err = ExpandVariables(value, variables)
		if err != nil {
			return Command{}, err
		}
		if value == "" {
			return Command{}, fmt.Errorf("wait-for requires a non-empty substring")
		}
		hasCancel := cancel != ""
		cancel, err = ExpandVariables(cancel, variables)
		if err != nil {
			return Command{}, err
		}
		if hasCancel && cancel == "" {
			return Command{}, fmt.Errorf("cancel-on requires a non-empty substring")
		}
		timeout, err := expandPositiveSeconds(timeoutText, variables, "wait-for timeout")
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindWaitFor, Match: value, Cancel: cancel, Timeout: timeout}, nil

	case "notify":
		title, value, err := parseNotifyArguments(arg)
		if err != nil {
			return Command{}, err
		}
		title, err = ExpandVariables(title, variables)
		if err != nil {
			return Command{}, err
		}
		value, err = ExpandVariables(value, variables)
		if err != nil {
			return Command{}, err
		}
		if value == "" {
			return Command{}, fmt.Errorf("notify requires a non-empty message")
		}
		if title == "" {
			title = "Praetor"
		}
		return Command{Kind: KindNotify, Title: title, Text: value}, nil

	case "repeat":
		command, match, cancel, maxText, err := parseRepeatArguments(arg)
		if err != nil {
			return Command{}, err
		}
		command, err = ExpandVariables(command, variables)
		if err != nil {
			return Command{}, err
		}
		match, err = ExpandVariables(match, variables)
		if err != nil {
			return Command{}, err
		}
		hasCancel := cancel != ""
		cancel, err = ExpandVariables(cancel, variables)
		if err != nil {
			return Command{}, err
		}
		if hasCancel && cancel == "" {
			return Command{}, fmt.Errorf("cancel-on requires a non-empty substring")
		}
		if command == "" || match == "" {
			return Command{}, fmt.Errorf("repeat command and success substring must not be empty")
		}
		max, err := expandPositiveInt(maxText, variables, "repeat max")
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindRepeat, Text: command, Match: match, Cancel: cancel, Max: max}, nil

	case "":
		return Command{}, fmt.Errorf("empty input directive")
	default:
		return Command{}, fmt.Errorf("unknown input directive %q", name)
	}
}

func expandPositiveSeconds(input string, variables map[string]string, name string) (time.Duration, error) {
	if input == "" {
		return 0, nil
	}
	expanded, err := ExpandVariables(input, variables)
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(expanded, 64)
	if err != nil || seconds <= 0 {
		return 0, fmt.Errorf("%s requires a positive number of seconds", name)
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration <= 0 {
		return 0, fmt.Errorf("%s is too small", name)
	}
	return duration, nil
}

func expandPositiveInt(input string, variables map[string]string, name string) (int, error) {
	if input == "" {
		return 0, nil
	}
	expanded, err := ExpandVariables(input, variables)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(expanded)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s requires a positive whole number", name)
	}
	return value, nil
}

func splitDirectiveHead(body string) (name, arg string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", ""
	}
	if i := strings.IndexAny(body, " \t"); i >= 0 {
		return strings.ToLower(body[:i]), strings.TrimSpace(body[i+1:])
	}
	return strings.ToLower(body), ""
}

func parseNotifyArguments(arg string) (title, message string, err error) {
	rest := strings.TrimSpace(arg)
	if rest == "" {
		return "", "", fmt.Errorf("notify requires a string")
	}
	if rest[0] != '"' {
		return "", rest, nil
	}
	first, rest, err := consumeQuoted(rest)
	if err != nil {
		return "", "", err
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", first, nil
	}
	if rest[0] != '"' {
		return "", "", fmt.Errorf("notify title must be followed by a quoted message")
	}
	second, rest, err := consumeQuoted(rest)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(rest) != "" {
		return "", "", fmt.Errorf("notify has unexpected trailing text")
	}
	if first == "" {
		return "", "", fmt.Errorf("notify requires a non-empty title")
	}
	return first, second, nil
}

func parseWaitForArguments(arg string) (match, cancel, timeout string, err error) {
	rest := strings.TrimSpace(arg)
	if rest == "" {
		return "", "", "", fmt.Errorf("wait-for requires a string")
	}
	if rest[0] != '"' {
		// Preserve the original convenient unquoted form. Optional clauses are
		// intentionally available only with a quoted match, avoiding ambiguity.
		return rest, "", "", nil
	}
	match, rest, err = consumeQuoted(rest)
	if err != nil {
		return "", "", "", err
	}
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		switch {
		case hasKeyword(rest, "cancel-on"):
			if cancel != "" {
				return "", "", "", fmt.Errorf("wait-for has more than one cancel-on clause")
			}
			rest = strings.TrimSpace(rest[len("cancel-on"):])
			if rest == "" || rest[0] != '"' {
				return "", "", "", fmt.Errorf("cancel-on requires a quoted substring")
			}
			cancel, rest, err = consumeQuoted(rest)
			if err != nil {
				return "", "", "", err
			}
			if cancel == "" {
				return "", "", "", fmt.Errorf("cancel-on requires a non-empty substring")
			}
		case hasKeyword(rest, "timeout"):
			if timeout != "" {
				return "", "", "", fmt.Errorf("wait-for has more than one timeout clause")
			}
			rest = strings.TrimSpace(rest[len("timeout"):])
			timeout, rest = consumeField(rest)
			if timeout == "" {
				return "", "", "", fmt.Errorf("wait-for timeout requires seconds")
			}
		default:
			return "", "", "", fmt.Errorf("wait-for accepts only cancel-on and timeout clauses")
		}
	}
	return match, cancel, timeout, nil
}

func parseRepeatArguments(arg string) (command, match, cancel, max string, err error) {
	rest := strings.TrimSpace(arg)
	if rest == "" || rest[0] != '"' {
		return "", "", "", "", fmt.Errorf("repeat requires a quoted command")
	}
	command, rest, err = consumeQuoted(rest)
	if err != nil {
		return "", "", "", "", err
	}
	rest = strings.TrimSpace(rest)
	if !hasKeyword(rest, "until") {
		return "", "", "", "", fmt.Errorf("repeat requires until followed by a quoted success substring")
	}
	rest = strings.TrimSpace(rest[len("until"):])
	if rest == "" || rest[0] != '"' {
		return "", "", "", "", fmt.Errorf("repeat requires a quoted success substring")
	}
	match, rest, err = consumeQuoted(rest)
	if err != nil {
		return "", "", "", "", err
	}
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		switch {
		case hasKeyword(rest, "cancel-on"):
			if cancel != "" {
				return "", "", "", "", fmt.Errorf("repeat has more than one cancel-on clause")
			}
			rest = strings.TrimSpace(rest[len("cancel-on"):])
			if rest == "" || rest[0] != '"' {
				return "", "", "", "", fmt.Errorf("cancel-on requires a quoted substring")
			}
			cancel, rest, err = consumeQuoted(rest)
			if err != nil {
				return "", "", "", "", err
			}
			if cancel == "" {
				return "", "", "", "", fmt.Errorf("cancel-on requires a non-empty substring")
			}
		case hasKeyword(rest, "max"):
			if max != "" {
				return "", "", "", "", fmt.Errorf("repeat has more than one max clause")
			}
			rest = strings.TrimSpace(rest[len("max"):])
			max, rest = consumeField(rest)
			if max == "" {
				return "", "", "", "", fmt.Errorf("repeat max requires an attempt count")
			}
		default:
			return "", "", "", "", fmt.Errorf("repeat accepts only cancel-on and max clauses after until")
		}
	}
	return command, match, cancel, max, nil
}

func consumeField(input string) (field, rest string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	if i := strings.IndexAny(input, " \t"); i >= 0 {
		return input[:i], input[i+1:]
	}
	return input, ""
}

func hasKeyword(input, keyword string) bool {
	return strings.EqualFold(firstField(input), keyword)
}

func firstField(input string) string {
	if i := strings.IndexAny(input, " \t"); i >= 0 {
		return input[:i]
	}
	return input
}

// consumeQuoted parses one double-quoted DSL string. Only quote and backslash
// are special; other backslashes remain available to the game command and to
// the outer \${ literal-variable escape.
func consumeQuoted(input string) (value, rest string, err error) {
	if input == "" || input[0] != '"' {
		return "", input, fmt.Errorf("expected a quoted string")
	}
	var out strings.Builder
	for i := 1; i < len(input); i++ {
		switch input[i] {
		case '"':
			return out.String(), input[i+1:], nil
		case '\\':
			if i+1 < len(input) && (input[i+1] == '"' || input[i+1] == '\\') {
				out.WriteByte(input[i+1])
				i++
				continue
			}
			out.WriteByte(input[i])
		default:
			out.WriteByte(input[i])
		}
	}
	return "", "", fmt.Errorf("unterminated quoted string")
}

// ExpandVariables replaces ${name} and ${name:fallback} references once
// without interpreting command separators. A fallback is used when the name
// is missing or its saved value is empty. Values and fallbacks are not scanned
// again, preventing recursive references and cycles. \${ emits a literal ${.
// This separate operation is used for multi-line input and /send files, where
// variables are supported but ;; and && must remain ordinary text.
func ExpandVariables(input string, variables map[string]string) (string, error) {
	var out strings.Builder
	out.Grow(len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+2 < len(input) && input[i+1] == '$' &&
			(input[i+2] == '{' || input[i+2] == '(') {
			out.WriteByte('$')
			out.WriteByte(input[i+2])
			i += 2
			continue
		}
		if input[i] != '$' || i+1 >= len(input) || input[i+1] != '{' {
			out.WriteByte(input[i])
			continue
		}
		end := strings.IndexByte(input[i+2:], '}')
		if end < 0 {
			return "", fmt.Errorf("unterminated variable reference at character %d", i+1)
		}
		end += i + 2
		reference := input[i+2 : end]
		name, fallback, hasFallback := strings.Cut(reference, ":")
		if !config.ValidVariableName(name) {
			return "", fmt.Errorf("invalid variable reference %q", name)
		}
		value, ok := variables[name]
		if (!ok || value == "") && hasFallback {
			value = fallback
		} else if !ok {
			return "", fmt.Errorf("unknown variable %q", name)
		}
		out.WriteString(value)
		i = end
	}
	return out.String(), nil
}
