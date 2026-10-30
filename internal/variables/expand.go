// Package variables implements Praetor's shared, deliberately non-recursive
// saved-variable syntax. It is independent of command parsing so match fields,
// notification templates, and command input can use one expansion contract.
package variables

import (
	"fmt"
	"strings"
)

// ErrorCode identifies why expansion failed without requiring callers to
// inspect user-facing error text.
type ErrorCode string

const (
	ErrorUnresolved   ErrorCode = "unresolved"
	ErrorInvalidName  ErrorCode = "invalid_name"
	ErrorUnterminated ErrorCode = "unterminated"
)

// ExpansionError describes a failed reference. ByteOffset is the zero-based
// byte position of the reference's '$' in the original input. Name is empty
// when an unterminated reference cannot be parsed safely.
type ExpansionError struct {
	Code       ErrorCode
	Name       string
	ByteOffset int
}

func (e *ExpansionError) Error() string {
	switch e.Code {
	case ErrorUnresolved:
		return fmt.Sprintf("unknown variable %q", e.Name)
	case ErrorInvalidName:
		return fmt.Sprintf("invalid variable reference %q", e.Name)
	case ErrorUnterminated:
		return fmt.Sprintf("unterminated variable reference at character %d", e.ByteOffset+1)
	default:
		return "variable expansion failed"
	}
}

// ValidName reports whether name is a portable saved-variable identifier.
// Names use ASCII letters or underscore initially and may then contain digits.
func ValidName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// Expand replaces ${name} and ${name:fallback} references once. A fallback is
// used when the name is missing or its saved value is empty. Values and
// fallbacks are not scanned again. \${ emits a literal ${ and \$( emits a
// literal $( so callers can preserve command-directive syntax as data.
func Expand(input string, values map[string]string) (string, error) {
	return expand(input, values, nil, nil)
}

// ExpandPreserving expands ordinary saved-variable references while leaving
// references selected by preserve byte-for-byte for a later expansion pass.
// The callback is responsible for recognizing only valid reserved names.
func ExpandPreserving(input string, values map[string]string, preserve func(string) bool) (string, error) {
	return expand(input, values, preserve, nil)
}

// ExpandReserved expands references using values and accepts names recognized
// by reserved in addition to ordinary saved-variable names. It is intended for
// small, read-only namespaces such as PraetorScript's live my.* values.
func ExpandReserved(input string, values map[string]string, reserved func(string) bool) (string, error) {
	return expand(input, values, nil, reserved)
}

// ExpandSelected expands only names selected by the callback and preserves all
// other references byte-for-byte. This keeps a second, reserved-namespace pass
// from recursively expanding ordinary saved-variable text inserted earlier.
func ExpandSelected(input string, values map[string]string, selected func(string) bool) (string, error) {
	return expand(input, values, func(name string) bool { return !selected(name) }, selected)
}

func expand(input string, values map[string]string, preserve, reserved func(string) bool) (string, error) {
	var out strings.Builder
	out.Grow(len(input))
	for i := 0; i < len(input); i++ {
		if input[i] == '\\' && i+2 < len(input) && input[i+1] == '$' &&
			(input[i+2] == '{' || input[i+2] == '(') {
			if preserve != nil && input[i+2] == '{' {
				if relativeEnd := strings.IndexByte(input[i+3:], '}'); relativeEnd >= 0 {
					end := i + 3 + relativeEnd
					reference := input[i+3 : end]
					name, _, _ := strings.Cut(reference, ":")
					if preserve(name) {
						out.WriteString(input[i : end+1])
						i = end
						continue
					}
				}
			}
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
			return "", &ExpansionError{Code: ErrorUnterminated, ByteOffset: i}
		}
		end += i + 2
		reference := input[i+2 : end]
		name, fallback, hasFallback := strings.Cut(reference, ":")
		if preserve != nil && preserve(name) {
			out.WriteString(input[i : end+1])
			i = end
			continue
		}
		if !ValidName(name) && (reserved == nil || !reserved(name)) {
			return "", &ExpansionError{Code: ErrorInvalidName, Name: name, ByteOffset: i}
		}
		value, ok := values[name]
		if (!ok || value == "") && hasFallback {
			value = fallback
		} else if !ok {
			return "", &ExpansionError{Code: ErrorUnresolved, Name: name, ByteOffset: i}
		}
		if preserve != nil {
			value = escapePreservedReferences(value, preserve)
		}
		out.WriteString(value)
		i = end
	}
	return out.String(), nil
}

func escapePreservedReferences(input string, preserve func(string) bool) string {
	var out strings.Builder
	for i := 0; i < len(input); i++ {
		if input[i] != '$' || i+1 >= len(input) || input[i+1] != '{' {
			out.WriteByte(input[i])
			continue
		}
		relativeEnd := strings.IndexByte(input[i+2:], '}')
		if relativeEnd < 0 {
			out.WriteByte(input[i])
			continue
		}
		end := i + 2 + relativeEnd
		reference := input[i+2 : end]
		name, _, _ := strings.Cut(reference, ":")
		if preserve(name) {
			out.WriteByte('\\')
		}
		out.WriteString(input[i : end+1])
		i = end
	}
	return out.String()
}
