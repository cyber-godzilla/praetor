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

// Expand splits one typed line on unescaped ;; separators and substitutes
// ${name} references in each resulting command. Splitting happens first, so a
// variable value containing ;; remains data and cannot inject extra commands.
func Expand(input string, variables map[string]string) ([]string, error) {
	parts := split(input)
	commands := make([]string, 0, len(parts))
	for _, part := range parts {
		command, err := substitute(part, variables)
		if err != nil {
			return nil, err
		}
		if len(parts) > 1 {
			command = strings.TrimSpace(command)
			if command == "" {
				continue
			}
		}
		commands = append(commands, command)
		if len(commands) > MaxCommands {
			return nil, fmt.Errorf("too many commands (maximum %d)", MaxCommands)
		}
	}
	return commands, nil
}

// split recognizes \;; as a literal double-semicolon. Other backslashes are
// preserved byte-for-byte for the game server.
func split(input string) []string {
	var parts []string
	var part strings.Builder
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+2 < len(input) && input[i+1] == ';' && input[i+2] == ';' {
			part.WriteString(";;")
			i += 2
			continue
		}
		if input[i] == ';' && i+1 < len(input) && input[i+1] == ';' {
			parts = append(parts, part.String())
			part.Reset()
			i++
			continue
		}
		part.WriteByte(input[i])
	}
	parts = append(parts, part.String())
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
