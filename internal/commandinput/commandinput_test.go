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
		want []string
	}{
		{name: "plain command", in: "look", want: []string{"look"}},
		{name: "single semicolon is ordinary text", in: "say yes; absolutely", want: []string{"say yes; absolutely"}},
		{name: "multiple commands", in: "north ;; look;;inventory", want: []string{"north", "look", "inventory"}},
		{name: "empty segments ignored", in: ";;north;;;;look;;", want: []string{"north", "look"}},
		{name: "escaped separator", in: `say one\;; two;;wave`, want: []string{"say one;; two", "wave"}},
		{name: "substitution", in: "kill ${target};;get ${weapon}", vars: map[string]string{"target": "scarred bandit", "weapon": "gladius"}, want: []string{"kill scarred bandit", "get gladius"}},
		{name: "variable cannot inject separator", in: "say ${message};;wave", vars: map[string]string{"message": "one;; two"}, want: []string{"say one;; two", "wave"}},
		{name: "non recursive variables", in: "say ${message}", vars: map[string]string{"message": "${target}", "target": "Marcus"}, want: []string{"say ${target}"}},
		{name: "literal variable syntax", in: `say \${target}`, vars: map[string]string{"target": "Marcus"}, want: []string{"say ${target}"}},
		{name: "blank input stays one command", in: "", want: []string{""}},
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
	commands, err := Expand(strings.Repeat("look;;", MaxCommands)+"look", nil)
	if err == nil {
		t.Fatalf("Expand returned %#v, nil; want maximum-command error", commands)
	}
	if commands != nil {
		t.Fatalf("commands = %#v, want nil on validation failure", commands)
	}
}

func TestExpandRejectsBadReferencesBeforeReturningCommands(t *testing.T) {
	tests := []string{
		"look;;kill ${missing}",
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
