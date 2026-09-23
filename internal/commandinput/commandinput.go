// Package commandinput expands the small, intentionally non-recursive syntax
// supported by Praetor's typed command line.
package commandinput

import (
	"fmt"
	"strings"

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

// Command is one fully expanded command and the separator that preceded it.
type Command struct {
	Text string
	Wait WaitMode
}

type part struct {
	text string
	wait WaitMode
}

// Expand splits one typed line on unescaped ;; and && separators and
// substitutes ${name} references in each resulting command. ;; waits for the
// normal pacing delay; && waits for a recognized unbusy game line. Splitting
// happens first, so variable values containing separators remain data and
// cannot inject extra commands.
func Expand(input string, variables map[string]string) ([]Command, error) {
	parts := split(input)
	commands := make([]Command, 0, len(parts))
	for _, part := range parts {
		text, err := substitute(part.text, variables)
		if err != nil {
			return nil, err
		}
		if len(parts) > 1 {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
		}
		wait := part.wait
		if len(commands) == 0 {
			wait = WaitNone
		}
		commands = append(commands, Command{Text: text, Wait: wait})
		if len(commands) > MaxCommands {
			return nil, fmt.Errorf("too many commands (maximum %d)", MaxCommands)
		}
	}
	return commands, nil
}

// split recognizes \;; and \&& as literal double-character sequences. Other
// backslashes are preserved byte-for-byte for the game server.
func split(input string) []part {
	var parts []part
	var text strings.Builder
	wait := WaitNone
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+2 < len(input) &&
			((input[i+1] == ';' && input[i+2] == ';') || (input[i+1] == '&' && input[i+2] == '&')) {
			text.WriteByte(input[i+1])
			text.WriteByte(input[i+2])
			i += 2
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
	parts = append(parts, part{text: text.String(), wait: wait})
	return parts
}

// substitute replaces ${name} once. Variable values are not scanned again,
// preventing recursive references and cycles. \${ emits a literal ${.
func substitute(input string, variables map[string]string) (string, error) {
	var out strings.Builder
	out.Grow(len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+2 < len(input) && input[i+1] == '$' && input[i+2] == '{' {
			out.WriteString("${")
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
		name := input[i+2 : end]
		if !config.ValidVariableName(name) {
			return "", fmt.Errorf("invalid variable reference %q", name)
		}
		value, ok := variables[name]
		if !ok {
			return "", fmt.Errorf("unknown variable %q", name)
		}
		out.WriteString(value)
		i = end
	}
	return out.String(), nil
}
