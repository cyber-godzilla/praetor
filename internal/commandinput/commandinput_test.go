package commandinput

import (
	"reflect"
	"strings"
	"testing"
)

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
		{name: "variable cannot inject separator", in: "say ${message}&&wave", vars: map[string]string{"message": "one&&two"}, want: []Command{{Text: "say one&&two"}, {Text: "wave", Wait: WaitUnbusy}}},
		{name: "non recursive variables", in: "say ${message}", vars: map[string]string{"message": "${target}", "target": "Marcus"}, want: []Command{{Text: "say ${target}"}}},
		{name: "literal variable syntax", in: `say \${target}`, vars: map[string]string{"target": "Marcus"}, want: []Command{{Text: "say ${target}"}}},
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
