// Package commandinput expands the small, intentionally non-recursive syntax
// supported by Praetor's user-submitted commands and text blocks.
package commandinput

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cyber-godzilla/praetor/internal/config"
	sharedvars "github.com/cyber-godzilla/praetor/internal/variables"
)

// MaxCommands limits one typed line so an accidental paste cannot enqueue an
// unbounded burst of commands.
const MaxCommands = 100

// SplitWords separates command arguments on whitespace while honoring
// double-quoted word groups. Quotes may wrap a whole argument or only part of
// one, so from:"2 sack" becomes the single argument "from:2 sack". Inside
// quotes, \" and \\ use the same escaping rules as PraetorScript directives;
// other backslashes are preserved.
func SplitWords(input string) ([]string, error) {
	var words []string
	var word strings.Builder
	inWord := false
	inQuote := false

	for i := 0; i < len(input); {
		if inQuote {
			switch input[i] {
			case '"':
				inQuote = false
				inWord = true
				i++
				continue
			case '\\':
				if i+1 < len(input) && (input[i+1] == '"' || input[i+1] == '\\') {
					word.WriteByte(input[i+1])
					i += 2
					continue
				}
			}
			word.WriteByte(input[i])
			inWord = true
			i++
			continue
		}

		switch input[i] {
		case '"':
			inQuote = true
			inWord = true
			i++
			continue
		}

		r, size := utf8.DecodeRuneInString(input[i:])
		if unicode.IsSpace(r) {
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
			i += size
			continue
		}
		word.WriteString(input[i : i+size])
		inWord = true
		i += size
	}

	if inQuote {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if inWord {
		words = append(words, word.String())
	}
	return words, nil
}

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
	KindSetVariable
	KindAddVariable
)

// Command is one parsed command and the separator that preceded it. Saved
// variables are expanded immediately; reserved my.* values remain late-bound.
type Command struct {
	Text             string
	Title            string
	Wait             WaitMode
	Kind             CommandKind
	Duration         time.Duration
	DurationTemplate string
	Timeout          time.Duration
	TimeoutTemplate  string
	Match            string
	MatchCondition   *ValueCondition
	Cancel           string
	CancelCondition  *ValueCondition
	Max              int
	MaxTemplate      string
	Count            int
	CountTemplate    string
	Variable         string
	Value            string
	Amount           float64
	AmountTemplate   string
}

// ValueCondition compares one live, read-only my.* value with a numeric
// operand. Operand may retain ${my.*} references for evaluation at runtime.
type ValueCondition struct {
	Field    string
	Operator string
	Operand  string
}

var liveValueFields = map[string]struct{}{
	"health":      {},
	"fatigue":     {},
	"encumbrance": {},
	"satiation":   {},
}

// IsLiveValueName reports whether name is one of PraetorScript's reserved,
// read-only my.* values.
func IsLiveValueName(name string) bool {
	if !strings.HasPrefix(name, "my.") {
		return false
	}
	_, ok := liveValueFields[strings.TrimPrefix(name, "my.")]
	return ok
}

type part struct {
	text string
	wait WaitMode
}

// Expand splits one typed line on unescaped ;; and && separators and
// substitutes saved ${name} or ${name:fallback} references in each resulting
// command while retaining my.* references for dispatch. ;; waits for the normal
// pacing delay; && waits for a recognized unbusy game line. Splitting happens
// first, so variable values and fallbacks
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
	text, err := expandPraetorVariables(part.text, variables)
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
		expanded, err := expandPraetorVariables(strings.TrimSpace(arg), variables)
		if err != nil {
			return Command{}, err
		}
		if containsLiveValueReference(expanded) {
			if err := validateLiveNumberTemplate(expanded, true, false, "wait"); err != nil {
				return Command{}, err
			}
			return Command{Kind: KindWait, DurationTemplate: expanded}, nil
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
		value, matchCondition, cancel, cancelCondition, timeoutText, err := parseWaitForArguments(arg)
		if err != nil {
			return Command{}, err
		}
		value, err = expandPraetorVariables(value, variables)
		if err != nil {
			return Command{}, err
		}
		if value == "" && matchCondition == nil {
			return Command{}, fmt.Errorf("wait-for requires a non-empty substring")
		}
		hasCancel := cancel != "" || cancelCondition != nil
		cancel, err = expandPraetorVariables(cancel, variables)
		if err != nil {
			return Command{}, err
		}
		if hasCancel && cancelCondition == nil && cancel == "" {
			return Command{}, fmt.Errorf("cancel-on requires a non-empty substring")
		}
		matchCondition, err = expandValueCondition(matchCondition, variables)
		if err != nil {
			return Command{}, fmt.Errorf("wait-for condition: %w", err)
		}
		cancelCondition, err = expandValueCondition(cancelCondition, variables)
		if err != nil {
			return Command{}, fmt.Errorf("cancel-on condition: %w", err)
		}
		timeout, timeoutTemplate, err := expandPositiveSeconds(timeoutText, variables, "wait-for timeout")
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindWaitFor, Match: value, MatchCondition: matchCondition, Cancel: cancel, CancelCondition: cancelCondition, Timeout: timeout, TimeoutTemplate: timeoutTemplate}, nil

	case "notify":
		title, value, err := parseNotifyArguments(arg)
		if err != nil {
			return Command{}, err
		}
		title, err = expandPraetorVariables(title, variables)
		if err != nil {
			return Command{}, err
		}
		value, err = expandPraetorVariables(value, variables)
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
		command, match, matchCondition, cancel, cancelCondition, maxText, countText, err := parseRepeatArguments(arg)
		if err != nil {
			return Command{}, err
		}
		command, err = expandPraetorVariables(command, variables)
		if err != nil {
			return Command{}, err
		}
		match, err = expandPraetorVariables(match, variables)
		if err != nil {
			return Command{}, err
		}
		hasCancel := cancel != "" || cancelCondition != nil
		cancel, err = expandPraetorVariables(cancel, variables)
		if err != nil {
			return Command{}, err
		}
		if hasCancel && cancelCondition == nil && cancel == "" {
			return Command{}, fmt.Errorf("cancel-on requires a non-empty substring")
		}
		if command == "" {
			return Command{}, fmt.Errorf("repeat command must not be empty")
		}
		if countText == "" && match == "" && matchCondition == nil {
			return Command{}, fmt.Errorf("repeat success substring must not be empty")
		}
		matchCondition, err = expandValueCondition(matchCondition, variables)
		if err != nil {
			return Command{}, fmt.Errorf("repeat until condition: %w", err)
		}
		cancelCondition, err = expandValueCondition(cancelCondition, variables)
		if err != nil {
			return Command{}, fmt.Errorf("cancel-on condition: %w", err)
		}
		max, maxTemplate, err := expandPositiveInt(maxText, variables, "repeat max")
		if err != nil {
			return Command{}, err
		}
		count, countTemplate, err := expandPositiveInt(countText, variables, "repeat count")
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindRepeat, Text: command, Match: match, MatchCondition: matchCondition, Cancel: cancel, CancelCondition: cancelCondition, Max: max, MaxTemplate: maxTemplate, Count: count, CountTemplate: countTemplate}, nil

	case "set-var":
		variable, value, err := parseSetVariableArguments(arg)
		if err != nil {
			return Command{}, err
		}
		value, err = expandPraetorVariables(value, variables)
		if err != nil {
			return Command{}, err
		}
		return Command{Kind: KindSetVariable, Variable: variable, Value: value}, nil

	case "add-var":
		variable, amountText, err := parseAddVariableArguments(arg)
		if err != nil {
			return Command{}, err
		}
		amountText, err = expandPraetorVariables(amountText, variables)
		if err != nil {
			return Command{}, err
		}
		if containsLiveValueReference(amountText) {
			if err := validateLiveNumberTemplate(amountText, false, false, "add-var amount"); err != nil {
				return Command{}, err
			}
			return Command{Kind: KindAddVariable, Variable: variable, AmountTemplate: amountText}, nil
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return Command{}, fmt.Errorf("add-var amount must be a finite number")
		}
		return Command{Kind: KindAddVariable, Variable: variable, Amount: amount}, nil

	case "":
		return Command{}, fmt.Errorf("empty input directive")
	default:
		return Command{}, fmt.Errorf("unknown input directive %q", name)
	}
}

func parseSetVariableArguments(arg string) (variable, value string, err error) {
	words, err := SplitWords(arg)
	if err != nil {
		return "", "", err
	}
	if len(words) < 2 {
		return "", "", fmt.Errorf("set-var requires a variable name and value")
	}
	if !config.ValidVariableName(words[0]) {
		return "", "", fmt.Errorf("invalid variable name %q", words[0])
	}
	return words[0], strings.Join(words[1:], " "), nil
}

func parseAddVariableArguments(arg string) (variable, amount string, err error) {
	words, err := SplitWords(arg)
	if err != nil {
		return "", "", err
	}
	if len(words) != 2 {
		return "", "", fmt.Errorf("add-var requires a variable name and amount")
	}
	if !config.ValidVariableName(words[0]) {
		return "", "", fmt.Errorf("invalid variable name %q", words[0])
	}
	return words[0], words[1], nil
}

func expandPositiveSeconds(input string, variables map[string]string, name string) (time.Duration, string, error) {
	if input == "" {
		return 0, "", nil
	}
	expanded, err := expandPraetorVariables(input, variables)
	if err != nil {
		return 0, "", err
	}
	if containsLiveValueReference(expanded) {
		if err := validateLiveNumberTemplate(expanded, true, false, name); err != nil {
			return 0, "", err
		}
		return 0, expanded, nil
	}
	seconds, err := strconv.ParseFloat(expanded, 64)
	if err != nil || seconds <= 0 {
		return 0, "", fmt.Errorf("%s requires a positive number of seconds", name)
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration <= 0 {
		return 0, "", fmt.Errorf("%s is too small", name)
	}
	return duration, "", nil
}

func expandPositiveInt(input string, variables map[string]string, name string) (int, string, error) {
	if input == "" {
		return 0, "", nil
	}
	expanded, err := expandPraetorVariables(input, variables)
	if err != nil {
		return 0, "", err
	}
	if containsLiveValueReference(expanded) {
		if err := validateLiveNumberTemplate(expanded, true, true, name); err != nil {
			return 0, "", err
		}
		return 0, expanded, nil
	}
	value, err := strconv.Atoi(expanded)
	if err != nil || value <= 0 {
		return 0, "", fmt.Errorf("%s requires a positive whole number", name)
	}
	return value, "", nil
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

func parseWaitForArguments(arg string) (match string, condition *ValueCondition, cancel string, cancelCondition *ValueCondition, timeout string, err error) {
	rest := strings.TrimSpace(arg)
	if rest == "" {
		return "", nil, "", nil, "", fmt.Errorf("wait-for requires a string or my.* condition")
	}
	if rest[0] != '"' && !strings.HasPrefix(strings.ToLower(firstField(rest)), "my.") {
		// Preserve the original convenient unquoted form. Optional clauses are
		// intentionally available only with a quoted match, avoiding ambiguity.
		return rest, nil, "", nil, "", nil
	}
	match, condition, rest, err = consumeMatcher(rest, "wait-for")
	if err != nil {
		return "", nil, "", nil, "", err
	}
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		switch {
		case hasKeyword(rest, "cancel-on"):
			if cancel != "" || cancelCondition != nil {
				return "", nil, "", nil, "", fmt.Errorf("wait-for has more than one cancel-on clause")
			}
			rest = strings.TrimSpace(rest[len("cancel-on"):])
			cancel, cancelCondition, rest, err = consumeMatcher(rest, "cancel-on")
			if err != nil {
				return "", nil, "", nil, "", err
			}
			if cancel == "" && cancelCondition == nil {
				return "", nil, "", nil, "", fmt.Errorf("cancel-on requires a non-empty substring or my.* condition")
			}
		case hasKeyword(rest, "timeout"):
			if timeout != "" {
				return "", nil, "", nil, "", fmt.Errorf("wait-for has more than one timeout clause")
			}
			rest = strings.TrimSpace(rest[len("timeout"):])
			timeout, rest = consumeField(rest)
			if timeout == "" {
				return "", nil, "", nil, "", fmt.Errorf("wait-for timeout requires seconds")
			}
		default:
			return "", nil, "", nil, "", fmt.Errorf("wait-for accepts only cancel-on and timeout clauses")
		}
	}
	return match, condition, cancel, cancelCondition, timeout, nil
}

func parseRepeatArguments(arg string) (command, match string, condition *ValueCondition, cancel string, cancelCondition *ValueCondition, max, count string, err error) {
	rest := strings.TrimSpace(arg)
	if rest == "" || rest[0] != '"' {
		return "", "", nil, "", nil, "", "", fmt.Errorf("repeat requires a quoted command")
	}
	command, rest, err = consumeQuoted(rest)
	if err != nil {
		return "", "", nil, "", nil, "", "", err
	}
	rest = strings.TrimSpace(rest)
	countMode := hasKeyword(rest, "count")
	if countMode {
		rest = strings.TrimSpace(rest[len("count"):])
		count, rest = consumeField(rest)
		if count == "" {
			return "", "", nil, "", nil, "", "", fmt.Errorf("repeat count requires an attempt count")
		}
	} else {
		if !hasKeyword(rest, "until") {
			return "", "", nil, "", nil, "", "", fmt.Errorf("repeat requires until followed by a quoted success substring or my.* condition, or count followed by an attempt count")
		}
		rest = strings.TrimSpace(rest[len("until"):])
		match, condition, rest, err = consumeMatcher(rest, "repeat until")
		if err != nil {
			return "", "", nil, "", nil, "", "", err
		}
	}
	for strings.TrimSpace(rest) != "" {
		rest = strings.TrimSpace(rest)
		switch {
		case hasKeyword(rest, "cancel-on"):
			if cancel != "" || cancelCondition != nil {
				return "", "", nil, "", nil, "", "", fmt.Errorf("repeat has more than one cancel-on clause")
			}
			rest = strings.TrimSpace(rest[len("cancel-on"):])
			cancel, cancelCondition, rest, err = consumeMatcher(rest, "cancel-on")
			if err != nil {
				return "", "", nil, "", nil, "", "", err
			}
			if cancel == "" && cancelCondition == nil {
				return "", "", nil, "", nil, "", "", fmt.Errorf("cancel-on requires a non-empty substring or my.* condition")
			}
		case hasKeyword(rest, "max"):
			if countMode {
				return "", "", nil, "", nil, "", "", fmt.Errorf("repeat count does not accept a max clause")
			}
			if max != "" {
				return "", "", nil, "", nil, "", "", fmt.Errorf("repeat has more than one max clause")
			}
			rest = strings.TrimSpace(rest[len("max"):])
			max, rest = consumeField(rest)
			if max == "" {
				return "", "", nil, "", nil, "", "", fmt.Errorf("repeat max requires an attempt count")
			}
		default:
			if countMode {
				return "", "", nil, "", nil, "", "", fmt.Errorf("repeat count accepts only an optional cancel-on clause")
			}
			return "", "", nil, "", nil, "", "", fmt.Errorf("repeat accepts only cancel-on and max clauses after until")
		}
	}
	return command, match, condition, cancel, cancelCondition, max, count, nil
}

func consumeMatcher(input, context string) (text string, condition *ValueCondition, rest string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, "", fmt.Errorf("%s requires a quoted substring or my.* condition", context)
	}
	if input[0] == '"' {
		text, rest, err = consumeQuoted(input)
		return text, nil, rest, err
	}
	if !strings.HasPrefix(strings.ToLower(firstField(input)), "my.") {
		return "", nil, "", fmt.Errorf("%s requires a quoted substring or my.* condition", context)
	}
	condition, rest, err = consumeValueCondition(input)
	return "", condition, rest, err
}

func consumeValueCondition(input string) (*ValueCondition, string, error) {
	fieldName, rest := consumeField(input)
	operator, rest := consumeField(rest)
	operand, rest := consumeField(rest)
	fieldName = strings.ToLower(fieldName)
	if !IsLiveValueName(fieldName) {
		return nil, "", fmt.Errorf("unknown live value %q", fieldName)
	}
	if !validComparisonOperator(operator) {
		return nil, "", fmt.Errorf("unsupported comparison operator %q", operator)
	}
	if operand == "" {
		return nil, "", fmt.Errorf("condition requires a numeric value")
	}
	return &ValueCondition{
		Field:    strings.TrimPrefix(fieldName, "my."),
		Operator: operator,
		Operand:  operand,
	}, rest, nil
}

func validComparisonOperator(operator string) bool {
	switch operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func expandValueCondition(condition *ValueCondition, variables map[string]string) (*ValueCondition, error) {
	if condition == nil {
		return nil, nil
	}
	expanded, err := expandPraetorVariables(condition.Operand, variables)
	if err != nil {
		return nil, err
	}
	if !containsLiveValueReference(expanded) {
		value, err := strconv.ParseFloat(expanded, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("comparison value must be a finite number")
		}
	}
	copy := *condition
	copy.Operand = expanded
	return &copy, nil
}

func containsLiveValueReference(input string) bool {
	for _, field := range []string{"health", "fatigue", "encumbrance", "satiation"} {
		if strings.Contains(input, "${my."+field+"}") || strings.Contains(input, "${my."+field+":") {
			return true
		}
	}
	return false
}

func validateLiveNumberTemplate(input string, positive, whole bool, name string) error {
	values := map[string]string{
		"my.health":      "1",
		"my.fatigue":     "1",
		"my.encumbrance": "1",
		"my.satiation":   "1",
	}
	expanded, err := ExpandLiveValues(input, values)
	if err != nil {
		return err
	}
	if whole {
		value, err := strconv.Atoi(expanded)
		if err != nil || (positive && value <= 0) {
			return fmt.Errorf("%s requires a positive whole number", name)
		}
		return nil
	}
	value, err := strconv.ParseFloat(expanded, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || (positive && value <= 0) {
		if positive {
			return fmt.Errorf("%s requires a positive number", name)
		}
		return fmt.Errorf("%s must be a finite number", name)
	}
	return nil
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
	return sharedvars.Expand(input, variables)
}

func expandPraetorVariables(input string, variables map[string]string) (string, error) {
	return sharedvars.ExpandPreserving(input, variables, IsLiveValueName)
}

// ExpandLiveValues resolves the reserved my.* references retained during the
// initial PraetorScript parse. Callers provide only values observed for the
// current game session; ordinary saved-variable references have already been
// expanded and are not accepted here.
func ExpandLiveValues(input string, values map[string]string) (string, error) {
	return sharedvars.ExpandSelected(input, values, IsLiveValueName)
}

// ResolveLiveCommand resolves late-bound my.* interpolation for the step about
// to execute. Value conditions remain live and are evaluated separately for
// every relevant SKOOT update.
func ResolveLiveCommand(command Command, values map[string]string) (Command, error) {
	var err error
	for _, target := range []*string{&command.Text, &command.Title, &command.Match, &command.Cancel, &command.Value} {
		*target, err = ExpandLiveValues(*target, values)
		if err != nil {
			return Command{}, err
		}
	}
	if command.DurationTemplate != "" {
		command.Duration, err = resolvePositiveSeconds(command.DurationTemplate, values, "wait")
		if err != nil {
			return Command{}, err
		}
		command.DurationTemplate = ""
	}
	if command.TimeoutTemplate != "" {
		command.Timeout, err = resolvePositiveSeconds(command.TimeoutTemplate, values, "wait-for timeout")
		if err != nil {
			return Command{}, err
		}
		command.TimeoutTemplate = ""
	}
	if command.MaxTemplate != "" {
		command.Max, err = resolvePositiveInt(command.MaxTemplate, values, "repeat max")
		if err != nil {
			return Command{}, err
		}
		command.MaxTemplate = ""
	}
	if command.CountTemplate != "" {
		command.Count, err = resolvePositiveInt(command.CountTemplate, values, "repeat count")
		if err != nil {
			return Command{}, err
		}
		command.CountTemplate = ""
	}
	if command.AmountTemplate != "" {
		expanded, expandErr := ExpandLiveValues(command.AmountTemplate, values)
		if expandErr != nil {
			return Command{}, expandErr
		}
		command.Amount, err = strconv.ParseFloat(expanded, 64)
		if err != nil || math.IsNaN(command.Amount) || math.IsInf(command.Amount, 0) {
			return Command{}, fmt.Errorf("add-var amount must be a finite number")
		}
		command.AmountTemplate = ""
	}
	return command, nil
}

func resolvePositiveSeconds(input string, values map[string]string, name string) (time.Duration, error) {
	expanded, err := ExpandLiveValues(input, values)
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseFloat(expanded, 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return 0, fmt.Errorf("%s requires a positive number of seconds", name)
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration <= 0 {
		return 0, fmt.Errorf("%s is too small", name)
	}
	return duration, nil
}

func resolvePositiveInt(input string, values map[string]string, name string) (int, error) {
	expanded, err := ExpandLiveValues(input, values)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(expanded)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s requires a positive whole number", name)
	}
	return value, nil
}

// EvaluateValueCondition compares a live field with its possibly interpolated
// numeric operand. An unobserved live value makes the condition false.
func EvaluateValueCondition(condition *ValueCondition, values map[string]string) (bool, error) {
	if condition == nil {
		return false, nil
	}
	leftText, ok := values["my."+condition.Field]
	if !ok {
		return false, nil
	}
	rightText, err := ExpandLiveValues(condition.Operand, values)
	if err != nil {
		if expansionErr, ok := err.(*sharedvars.ExpansionError); ok && expansionErr.Code == sharedvars.ErrorUnresolved {
			return false, nil
		}
		return false, err
	}
	left, leftErr := strconv.ParseFloat(leftText, 64)
	right, rightErr := strconv.ParseFloat(rightText, 64)
	if leftErr != nil || rightErr != nil || math.IsNaN(left) || math.IsNaN(right) || math.IsInf(left, 0) || math.IsInf(right, 0) {
		return false, fmt.Errorf("condition values must be finite numbers")
	}
	switch condition.Operator {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case "<":
		return left < right, nil
	case "<=":
		return left <= right, nil
	case ">":
		return left > right, nil
	case ">=":
		return left >= right, nil
	default:
		return false, fmt.Errorf("unsupported comparison operator %q", condition.Operator)
	}
}
