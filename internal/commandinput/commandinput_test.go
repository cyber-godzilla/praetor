package commandinput

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "plain fields", in: "  /mode  unlock_all\tfrom:wagon ", want: []string{"/mode", "unlock_all", "from:wagon"}},
		{name: "quoted word", in: `/mode unlock_all "2 sack"`, want: []string{"/mode", "unlock_all", "2 sack"}},
		{name: "quoted option value", in: `/mode unlock_all from:"2 sack" reject:"worn large sack"`, want: []string{"/mode", "unlock_all", "from:2 sack", "reject:worn large sack"}},
		{name: "adjacent fragments", in: `one" two "three`, want: []string{"one two three"}},
		{name: "empty quoted word", in: `one "" three`, want: []string{"one", "", "three"}},
		{name: "quoted escapes", in: `say:"go \"now\"" path:"C:\\scripts"`, want: []string{`say:go "now"`, `path:C:\scripts`}},
		{name: "other quoted backslash preserved", in: `value:"a\tb"`, want: []string{`value:a\tb`}},
		{name: "unicode whitespace", in: "one\u2003two", want: []string{"one", "two"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitWords(tt.in)
			if err != nil {
				t.Fatalf("SplitWords: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("words = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSplitWordsRejectsUnterminatedQuote(t *testing.T) {
	got, err := SplitWords(`/mode unlock_all from:"2 sack`)
	if err == nil {
		t.Fatalf("SplitWords returned %#v, nil; want error", got)
	}
	if got != nil {
		t.Fatalf("words = %#v, want nil on validation failure", got)
	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		vars map[string]string
		want []Command
	}{
		{name: "plain command", in: "look", want: []Command{{Text: "look"}}},
		{name: "single semicolon is ordinary text", in: "say yes; absolutely", want: []Command{{Text: "say yes; absolutely"}}},
		{name: "multiple paced commands", in: "north ;; look;;inventory", want: []Command{{Text: "north"}, {Text: "look", Wait: WaitDelay}, {Text: "inventory", Wait: WaitDelay}}},
		{name: "unbusy commands", in: "stand&&climb wall&&look", want: []Command{{Text: "stand"}, {Text: "climb wall", Wait: WaitUnbusy}, {Text: "look", Wait: WaitUnbusy}}},
		{name: "mixed separators", in: "stand;;climb wall&&look", want: []Command{{Text: "stand"}, {Text: "climb wall", Wait: WaitDelay}, {Text: "look", Wait: WaitUnbusy}}},
		{name: "empty segments ignored", in: ";;north;;;;look&&", want: []Command{{Text: "north"}, {Text: "look", Wait: WaitDelay}}},
		{name: "escaped separators", in: `say one\;; two and three\&& four;;wave`, want: []Command{{Text: "say one;; two and three&& four"}, {Text: "wave", Wait: WaitDelay}}},
		{name: "single ampersand is ordinary text", in: "say salt & pepper", want: []Command{{Text: "say salt & pepper"}}},
		{name: "substitution", in: "kill ${target}&&get ${weapon}", vars: map[string]string{"target": "scarred bandit", "weapon": "gladius"}, want: []Command{{Text: "kill scarred bandit"}, {Text: "get gladius", Wait: WaitUnbusy}}},
		{name: "populated variable wins over fallback", in: "count ${count:25}", vars: map[string]string{"count": "10"}, want: []Command{{Text: "count 10"}}},
		{name: "zero variable wins over fallback", in: "count ${count:25}", vars: map[string]string{"count": "0"}, want: []Command{{Text: "count 0"}}},
		{name: "whitespace variable wins over fallback", in: "say [${message:fallback}]", vars: map[string]string{"message": " "}, want: []Command{{Text: "say [ ]"}}},
		{name: "missing variable uses fallback", in: "count ${count:25}", want: []Command{{Text: "count 25"}}},
		{name: "empty variable uses fallback", in: "count ${count:25}", vars: map[string]string{"count": ""}, want: []Command{{Text: "count 25"}}},
		{name: "fallback keeps colons", in: "say ${message:time: 25}", want: []Command{{Text: "say time: 25"}}},
		{name: "fallback cannot inject separator", in: "say ${message:one;;two}&&wave", want: []Command{{Text: "say one;;two"}, {Text: "wave", Wait: WaitUnbusy}}},
		{name: "fallback cannot inject directive", in: "say ${message:$(wait 5)}", want: []Command{{Text: "say $(wait 5)"}}},
		{name: "variable cannot inject separator", in: "say ${message}&&wave", vars: map[string]string{"message": "one&&two"}, want: []Command{{Text: "say one&&two"}, {Text: "wave", Wait: WaitUnbusy}}},
		{name: "non recursive variables", in: "say ${message}", vars: map[string]string{"message": "${target}", "target": "Marcus"}, want: []Command{{Text: "say ${target}"}}},
		{name: "live value remains late bound", in: "say Health: ${my.health}", want: []Command{{Text: "say Health: ${my.health}"}}},
		{name: "live value fallback remains late bound", in: "say Health: ${my.health:unknown}", want: []Command{{Text: "say Health: ${my.health:unknown}"}}},
		{name: "literal variable syntax", in: `say \${target}`, vars: map[string]string{"target": "Marcus"}, want: []Command{{Text: "say ${target}"}}},
		{name: "wait directive", in: `look;;$(wait 1.25);;inventory`, want: []Command{{Text: "look"}, {Kind: KindWait, Duration: 1250 * time.Millisecond, Wait: WaitDelay}, {Text: "inventory", Wait: WaitDelay}}},
		{name: "wait variable", in: `$(wait ${delay})`, vars: map[string]string{"delay": "0.5"}, want: []Command{{Kind: KindWait, Duration: 500 * time.Millisecond}}},
		{name: "wait fallback", in: `$(wait ${delay:0.5})`, want: []Command{{Kind: KindWait, Duration: 500 * time.Millisecond}}},
		{name: "wait-for quoted separators", in: `$(wait-for "the guard says ;; wait && listen")&&look`, want: []Command{{Kind: KindWaitFor, Match: "the guard says ;; wait && listen"}, {Text: "look", Wait: WaitUnbusy}}},
		{name: "wait-for unquoted remainder", in: `$(wait-for the latch clicks);;open door`, want: []Command{{Kind: KindWaitFor, Match: "the latch clicks"}, {Text: "open door", Wait: WaitDelay}}},
		{name: "wait-for bounded", in: `$(wait-for "The gate opens" timeout 30)`, want: []Command{{Kind: KindWaitFor, Match: "The gate opens", Timeout: 30 * time.Second}}},
		{name: "wait-for cancel and variable timeout", in: `$(wait-for "open" cancel-on "locked" timeout ${limit:2.5})`, want: []Command{{Kind: KindWaitFor, Match: "open", Cancel: "locked", Timeout: 2500 * time.Millisecond}}},
		{name: "wait-for clauses accept either order", in: `$(wait-for "open" timeout 4 cancel-on "locked")`, want: []Command{{Kind: KindWaitFor, Match: "open", Cancel: "locked", Timeout: 4 * time.Second}}},
		{name: "wait-for live condition", in: `$(wait-for my.health >= 75)`, want: []Command{{Kind: KindWaitFor, MatchCondition: &ValueCondition{Field: "health", Operator: ">=", Operand: "75"}}}},
		{name: "wait-for live condition with saved threshold", in: `$(wait-for my.fatigue < ${target:20} timeout 30)`, want: []Command{{Kind: KindWaitFor, MatchCondition: &ValueCondition{Field: "fatigue", Operator: "<", Operand: "20"}, Timeout: 30 * time.Second}}},
		{name: "wait-for text with live cancellation", in: `$(wait-for "safe" cancel-on my.health <= 25)`, want: []Command{{Kind: KindWaitFor, Match: "safe", CancelCondition: &ValueCondition{Field: "health", Operator: "<=", Operand: "25"}}}},
		{name: "wait-for live condition with text cancellation", in: `$(wait-for my.health >= 75 cancel-on "poisoned")`, want: []Command{{Kind: KindWaitFor, MatchCondition: &ValueCondition{Field: "health", Operator: ">=", Operand: "75"}, Cancel: "poisoned"}}},
		{name: "notify directive", in: `$(notify "Training complete")`, want: []Command{{Kind: KindNotify, Title: "Praetor", Text: "Training complete"}}},
		{name: "notify titled", in: `$(notify "Training" "Complete")`, want: []Command{{Kind: KindNotify, Title: "Training", Text: "Complete"}}},
		{name: "notify title and message expand variables", in: `$(notify "${activity:Training}" "${result:Complete}")`, want: []Command{{Kind: KindNotify, Title: "Training", Text: "Complete"}}},
		{name: "notify directive expands fallback", in: `$(notify "Training ${result:complete}")`, want: []Command{{Kind: KindNotify, Title: "Praetor", Text: "Training complete"}}},
		{name: "notify keeps live values late bound", in: `$(notify "Vitals" "Health ${my.health}, fatigue ${my.fatigue:unknown}")`, want: []Command{{Kind: KindNotify, Title: "Vitals", Text: "Health ${my.health}, fatigue ${my.fatigue:unknown}"}}},
		{name: "directive names and repeat keywords ignore case", in: `$(REPEAT "climb" UNTIL "done" CANCEL-ON "failed")`, want: []Command{{Kind: KindRepeat, Text: "climb", Match: "done", Cancel: "failed"}}},
		{name: "repeat until success", in: `$(repeat "climb ${target}" until "You reach ${target}")`, vars: map[string]string{"target": "the wall"}, want: []Command{{Kind: KindRepeat, Text: "climb the wall", Match: "You reach the wall"}}},
		{name: "repeat arguments use fallbacks", in: `$(repeat "climb ${target:wall}" until "You reach ${success:the top}")`, want: []Command{{Kind: KindRepeat, Text: "climb wall", Match: "You reach the top"}}},
		{name: "repeat with cancellation", in: `$(repeat "climb wall" until "You reach the top" cancel-on "You fall")`, want: []Command{{Kind: KindRepeat, Text: "climb wall", Match: "You reach the top", Cancel: "You fall"}}},
		{name: "repeat until live condition", in: `$(repeat "rest" until my.fatigue <= 20)`, want: []Command{{Kind: KindRepeat, Text: "rest", MatchCondition: &ValueCondition{Field: "fatigue", Operator: "<=", Operand: "20"}}}},
		{name: "repeat text until with live cancellation", in: `$(repeat "advance" until "You arrive" cancel-on my.health <= 25 max 10)`, want: []Command{{Kind: KindRepeat, Text: "advance", Match: "You arrive", CancelCondition: &ValueCondition{Field: "health", Operator: "<=", Operand: "25"}, Max: 10}}},
		{name: "repeat bounded", in: `$(repeat "climb wall" until "You reach the top" max 10)`, want: []Command{{Kind: KindRepeat, Text: "climb wall", Match: "You reach the top", Max: 10}}},
		{name: "repeat clauses accept either order", in: `$(repeat "climb" until "done" max ${tries:3} cancel-on "failed")`, want: []Command{{Kind: KindRepeat, Text: "climb", Match: "done", Cancel: "failed", Max: 3}}},
		{name: "repeat exact count", in: `$(repeat "search" count 5)`, want: []Command{{Kind: KindRepeat, Text: "search", Count: 5}}},
		{name: "repeat count uses variable fallback and cancellation", in: `$(repeat "search" count ${tries:3} cancel-on "You find nothing")`, want: []Command{{Kind: KindRepeat, Text: "search", Cancel: "You find nothing", Count: 3}}},
		{name: "repeat count keywords ignore case", in: `$(REPEAT "search" COUNT 2 CANCEL-ON "stop")`, want: []Command{{Kind: KindRepeat, Text: "search", Cancel: "stop", Count: 2}}},
		{name: "repeat text keeps punctuation", in: `$(repeat "say left | right && steady" until "Result | success" cancel-on "Result;;cancel")`, want: []Command{{Kind: KindRepeat, Text: "say left | right && steady", Match: "Result | success", Cancel: "Result;;cancel"}}},
		{name: "set variable", in: `$(set-var speaker "A man in a hooded cloak")`, want: []Command{{Kind: KindSetVariable, Variable: "speaker", Value: "A man in a hooded cloak"}}},
		{name: "set variable expands value", in: `$(set-var speaker ${target})`, vars: map[string]string{"target": "Apula"}, want: []Command{{Kind: KindSetVariable, Variable: "speaker", Value: "Apula"}}},
		{name: "set variable keeps live value late bound", in: `$(set-var recorded_health ${my.health})`, want: []Command{{Kind: KindSetVariable, Variable: "recorded_health", Value: "${my.health}"}}},
		{name: "set variable empty value", in: `$(set-var speaker "")`, want: []Command{{Kind: KindSetVariable, Variable: "speaker", Value: ""}}},
		{name: "add variable", in: `$(add-var counter 2.5)`, want: []Command{{Kind: KindAddVariable, Variable: "counter", Amount: 2.5}}},
		{name: "add variable expands amount", in: `$(add-var counter ${amount:-3})`, want: []Command{{Kind: KindAddVariable, Variable: "counter", Amount: -3}}},
		{name: "add variable keeps live amount late bound", in: `$(add-var counter ${my.health})`, want: []Command{{Kind: KindAddVariable, Variable: "counter", AmountTemplate: "${my.health}"}}},
		{name: "variable directives ignore case", in: `$(SET-VAR counter 1);;$(ADD-VAR counter 2)`, want: []Command{{Kind: KindSetVariable, Variable: "counter", Value: "1"}, {Kind: KindAddVariable, Variable: "counter", Amount: 2, Wait: WaitDelay}}},
		{name: "quoted escapes", in: `$(repeat "say \"go\"" until "He says \"go\"")`, want: []Command{{Kind: KindRepeat, Text: `say "go"`, Match: `He says "go"`}}},
		{name: "quoted backslash escape", in: `$(notify "C:\\scripts")`, want: []Command{{Kind: KindNotify, Title: "Praetor", Text: `C:\scripts`}}},
		{name: "literal directive syntax", in: `say \$(wait 2)`, want: []Command{{Text: "say $(wait 2)"}}},
		{name: "blank input stays one command", in: "", want: []Command{{Text: ""}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Expand(tt.in, tt.vars)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("commands = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestExpandRejectsInvalidDirectivesAtomically(t *testing.T) {
	tests := []string{
		`look;;$(wait 0)`,
		`look;;$(wait nope)`,
		`look;;$(wait-for "")`,
		`look;;$(wait-for "ready" timeout 0)`,
		`look;;$(wait-for "ready" timeout nope)`,
		`look;;$(wait-for "ready" cancel-on "")`,
		`look;;$(wait-for "ready" cancel-on "${cancel}")`,
		`look;;$(wait-for my.mana >= 10)`,
		`look;;$(wait-for my.health = 10)`,
		`look;;$(wait-for my.health >= nope)`,
		`look;;$(wait-for my.health >= 10 cancel-on my.mana < 5)`,
		`look;;$(wait-for "ready" cancel-on my.health)`,
		`look;;$(notify "")`,
		`look;;$(notify "" "message")`,
		`look;;$(notify "title" "")`,
		`look;;$(notify "title" message)`,
		`look;;$(repeat climb until "done")`,
		`look;;$(repeat "climb" "done")`,
		`look;;$(repeat "climb" until done)`,
		`look;;$(repeat "climb" until "done" cancel-on nope)`,
		`look;;$(repeat "climb" until "done" cancel-on "")`,
		`look;;$(repeat "climb" until "done" cancel-on "${cancel}")`,
		`look;;$(repeat "climb" until my.mana > 5)`,
		`look;;$(repeat "climb" until my.health about 5)`,
		`look;;$(repeat "climb" until my.health > many)`,
		`look;;$(repeat "climb" until "done" max 0)`,
		`look;;$(repeat "climb" until "done" max 1.5)`,
		`look;;$(repeat "climb" count)`,
		`look;;$(repeat "climb" count 0)`,
		`look;;$(repeat "climb" count 1.5)`,
		`look;;$(repeat "climb" count nope)`,
		`look;;$(repeat "climb" count 2 max 3)`,
		`look;;$(repeat "climb" count 2 until "done")`,
		`look;;$(repeat "climb" count 2 cancel-on nope)`,
		`look;;$(repeat "climb" count 2 cancel-on "")`,
		`look;;$(repeat "climb" until "done" count 2)`,
		`look;;$(set-var)`,
		`look;;$(set-var only_name)`,
		`look;;$(set-var bad-name value)`,
		`look;;$(add-var)`,
		`look;;$(add-var counter)`,
		`look;;$(add-var bad-name 1)`,
		`look;;$(add-var counter nope)`,
		`look;;$(add-var counter 1 extra)`,
		`look;;$(add-var counter NaN)`,
		`look;;$(add-var counter +Inf)`,
		`look;;$(unknown "value")`,
		`look $(wait 2)`,
		`look;;$(notify "done" trailing)`,
		`look;;$(wait-for "never closes)`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			commands, err := Expand(input, nil)
			if err == nil {
				t.Fatalf("Expand(%q) = %#v, nil; want error", input, commands)
			}
			if commands != nil {
				t.Fatalf("commands = %#v, want nil on validation failure", commands)
			}
		})
	}
}

func TestResolveLiveCommand(t *testing.T) {
	commands, err := Expand(`say health ${my.health}`, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got, err := ResolveLiveCommand(commands[0], map[string]string{"my.health": "70"})
	if err != nil {
		t.Fatalf("ResolveLiveCommand: %v", err)
	}
	if got.Text != "say health 70" {
		t.Fatalf("Text = %q, want %q", got.Text, "say health 70")
	}
}

func TestResolveLiveCommandResolvesDirectiveFieldsAndNumericArguments(t *testing.T) {
	values := map[string]string{
		"my.health":      "70",
		"my.fatigue":     "2",
		"my.encumbrance": "3",
		"my.satiation":   "4",
	}
	tests := []struct {
		input string
		check func(t *testing.T, command Command)
	}{
		{input: `$(notify "Health ${my.health}" "Fatigue ${my.fatigue}")`, check: func(t *testing.T, command Command) {
			if command.Title != "Health 70" || command.Text != "Fatigue 2" {
				t.Fatalf("notification = %#v", command)
			}
		}},
		{input: `$(set-var snapshot ${my.health})`, check: func(t *testing.T, command Command) {
			if command.Value != "70" {
				t.Fatalf("set-var value = %q", command.Value)
			}
		}},
		{input: `$(add-var total ${my.fatigue})`, check: func(t *testing.T, command Command) {
			if command.Amount != 2 || command.AmountTemplate != "" {
				t.Fatalf("add-var = %#v", command)
			}
		}},
		{input: `$(wait ${my.fatigue})`, check: func(t *testing.T, command Command) {
			if command.Duration != 2*time.Second || command.DurationTemplate != "" {
				t.Fatalf("wait = %#v", command)
			}
		}},
		{input: `$(wait-for "ready" timeout ${my.fatigue})`, check: func(t *testing.T, command Command) {
			if command.Timeout != 2*time.Second || command.TimeoutTemplate != "" {
				t.Fatalf("wait-for = %#v", command)
			}
		}},
		{input: `$(repeat "rest ${my.health}" until "done" max ${my.encumbrance})`, check: func(t *testing.T, command Command) {
			if command.Text != "rest 70" || command.Max != 3 || command.MaxTemplate != "" {
				t.Fatalf("repeat max = %#v", command)
			}
		}},
		{input: `$(repeat "search" count ${my.satiation})`, check: func(t *testing.T, command Command) {
			if command.Count != 4 || command.CountTemplate != "" {
				t.Fatalf("repeat count = %#v", command)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			commands, err := Expand(test.input, nil)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}
			resolved, err := ResolveLiveCommand(commands[0], values)
			if err != nil {
				t.Fatalf("ResolveLiveCommand: %v", err)
			}
			test.check(t, resolved)
		})
	}
}

func TestResolveLiveCommandFallbackAndLiteralEscape(t *testing.T) {
	commands, err := Expand(`say ${my.health:unknown} \${my.fatigue}`, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	resolved, err := ResolveLiveCommand(commands[0], nil)
	if err != nil {
		t.Fatalf("ResolveLiveCommand: %v", err)
	}
	if resolved.Text != `say unknown ${my.fatigue}` {
		t.Fatalf("Text = %q", resolved.Text)
	}
}

func TestResolveLiveCommandDoesNotRecursivelyExpandSavedVariableValue(t *testing.T) {
	commands, err := Expand(`say ${message}`, map[string]string{"message": `${my.health}`})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	resolved, err := ResolveLiveCommand(commands[0], map[string]string{"my.health": "70"})
	if err != nil {
		t.Fatalf("ResolveLiveCommand: %v", err)
	}
	if resolved.Text != `say ${my.health}` {
		t.Fatalf("Text = %q, want a literal non-recursive reference", resolved.Text)
	}
}

func TestEvaluateValueCondition(t *testing.T) {
	values := map[string]string{"my.health": "70", "my.fatigue": "20"}
	for _, test := range []struct {
		condition ValueCondition
		want      bool
	}{
		{condition: ValueCondition{Field: "health", Operator: "==", Operand: "70"}, want: true},
		{condition: ValueCondition{Field: "health", Operator: "!=", Operand: "60"}, want: true},
		{condition: ValueCondition{Field: "fatigue", Operator: "<", Operand: "21"}, want: true},
		{condition: ValueCondition{Field: "fatigue", Operator: "<=", Operand: "20"}, want: true},
		{condition: ValueCondition{Field: "health", Operator: ">", Operand: "69"}, want: true},
		{condition: ValueCondition{Field: "health", Operator: ">=", Operand: "${my.fatigue}"}, want: true},
		{condition: ValueCondition{Field: "satiation", Operator: ">=", Operand: "1"}, want: false},
	} {
		got, err := EvaluateValueCondition(&test.condition, values)
		if err != nil {
			t.Fatalf("EvaluateValueCondition(%#v): %v", test.condition, err)
		}
		if got != test.want {
			t.Errorf("EvaluateValueCondition(%#v) = %t, want %t", test.condition, got, test.want)
		}
	}
}

func TestExpandVariablesPreservesSeparatorsAndNewlines(t *testing.T) {
	got, err := ExpandVariables("say ${target};;look\n${target}&&wait", map[string]string{"target": "scarred bandit"})
	if err != nil {
		t.Fatalf("ExpandVariables: %v", err)
	}
	want := "say scarred bandit;;look\nscarred bandit&&wait"
	if got != want {
		t.Fatalf("expanded text = %q, want %q", got, want)
	}
}

func TestExpandVariablesFallbacksApplyToMultilineText(t *testing.T) {
	got, err := ExpandVariables("say ${message:hello there}\ncount ${count:25}", map[string]string{"message": "", "count": "3"})
	if err != nil {
		t.Fatalf("ExpandVariables: %v", err)
	}
	want := "say hello there\ncount 3"
	if got != want {
		t.Fatalf("expanded text = %q, want %q", got, want)
	}
}

func TestExpandVariablesEscapesDirectiveSyntaxWithoutInterpretingIt(t *testing.T) {
	got, err := ExpandVariables(`say \$(wait 2) and \${target}`, map[string]string{"target": "Marcus"})
	if err != nil {
		t.Fatalf("ExpandVariables: %v", err)
	}
	want := `say $(wait 2) and ${target}`
	if got != want {
		t.Fatalf("expanded text = %q, want %q", got, want)
	}
}

func TestExpandAcceptsExactlyMaximumCommands(t *testing.T) {
	input := strings.Repeat("look;;", MaxCommands-1) + "look"
	commands, err := Expand(input, nil)
	if err != nil {
		t.Fatalf("Expand at maximum: %v", err)
	}
	if len(commands) != MaxCommands {
		t.Fatalf("len(commands) = %d, want %d", len(commands), MaxCommands)
	}
}

func TestExpandRejectsMoreThanMaximumCommands(t *testing.T) {
	for _, separator := range []string{";;", "&&"} {
		commands, err := Expand(strings.Repeat("look"+separator, MaxCommands)+"look", nil)
		if err == nil {
			t.Fatalf("Expand with %q returned %#v, nil; want maximum-command error", separator, commands)
		}
		if commands != nil {
			t.Fatalf("commands = %#v, want nil on validation failure", commands)
		}
	}
}

func TestExpandRejectsBadReferencesBeforeReturningCommands(t *testing.T) {
	tests := []string{
		"look;;kill ${missing}",
		"look&&kill ${missing}",
		"say ${bad-name}",
		"say ${target",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			commands, err := Expand(input, map[string]string{"target": "Marcus"})
			if err == nil {
				t.Fatalf("Expand(%q) = %#v, nil; want error", input, commands)
			}
			if commands != nil {
				t.Fatalf("commands = %#v, want nil on validation failure", commands)
			}
		})
	}
}
