package client

import (
	"reflect"
	"testing"
)

func TestLuaVariablesReadWriteAndPersistThroughClient(t *testing.T) {
	c, _ := newLuaPraetorScriptClient(t, map[string]string{
		"variable_writer": `
local M = {}
M.on_start = function(args)
    variables.set("crafting_material", args[1])
    variables.set("copied_target", variables.get("target"))
end
M.reactions = {}
return M
`,
	})
	c.SetInputVariables(map[string]string{"target": "scarred bandit"})
	var saved map[string]string
	c.SetInputVariableSaver(func(variables map[string]string) error {
		saved = cloneVariables(variables)
		return nil
	})

	c.Engine.SetMode("variable_writer", []string{"gold"})
	want := map[string]string{
		"target":            "scarred bandit",
		"crafting_material": "gold",
		"copied_target":     "scarred bandit",
	}
	if got := c.inputVariableSnapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("live variables = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(saved, want) {
		t.Fatalf("saved variables = %#v, want %#v", saved, want)
	}
}
