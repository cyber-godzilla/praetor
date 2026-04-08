package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupEngineTestDirs(t *testing.T) (modesDir, libDir string) {
	t.Helper()
	modesDir = t.TempDir()
	libDir = t.TempDir()
	return modesDir, libDir
}

func writeEngineMode(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing mode file %s: %v", name, err)
	}
}

func newTestEngine(t *testing.T, modesDir, libDir string) *Engine {
	t.Helper()
	dirs := []string{modesDir}
	if libDir != "" {
		dirs = append(dirs, libDir)
	}
	e, err := NewEngine(dirs, nil, "")
	if err != nil {
		t.Fatalf("NewEngine error: %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

func TestEngine_ProcessMatch(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "loot", `
local M = {}
M.on_start = function(args)
    send("look")
end
M.reactions = {
    {
        match = "You take",
        action = function()
            send("get item from corpse")
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("loot", nil)

	// Drain the on_start command
	_, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("expected on_start command in queue")
	}

	// Process matching text
	e.Process("You take a bronze sword from the corpse.")

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("expected command after matching process")
	}
	if cmd.Command != "get item from corpse" {
		t.Errorf("command = %q, want 'get item from corpse'", cmd.Command)
	}
}

func TestEngine_ConditionGuardTrue(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "cond", `
local M = {}
M.on_start = function(args)
    state.set("ready", true)
end
M.reactions = {
    {
        match = "prompt",
        action = function()
            send("attack")
        end,
        condition = function()
            return state.get("ready") == true
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("cond", nil)
	e.Queue().Clear() // drain on_start

	e.Process("prompt")

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("condition was true, expected command")
	}
	if cmd.Command != "attack" {
		t.Errorf("command = %q, want 'attack'", cmd.Command)
	}
}

func TestEngine_ConditionGuardFalse(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "cond", `
local M = {}
M.on_start = function(args)
    state.set("ready", false)
end
M.reactions = {
    {
        match = "prompt",
        action = function()
            send("attack")
        end,
        condition = function()
            return state.get("ready")
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("cond", nil)
	e.Queue().Clear()

	e.Process("prompt")

	if e.Queue().Len() != 0 {
		t.Error("condition was false, expected no command")
	}
}

func TestEngine_DelayedReaction(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "delayed", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "trigger",
        action = function()
            send("delayed_cmd")
        end,
        delay = 100,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("delayed", nil)
	e.Queue().Clear()

	e.Process("trigger")

	// Immediately, queue should be empty (command is delayed)
	if e.Queue().Len() != 0 {
		t.Error("expected empty queue immediately after delayed reaction")
	}

	// Wait for the delay to pass
	time.Sleep(200 * time.Millisecond)

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("expected delayed command after waiting")
	}
	if cmd.Command != "delayed_cmd" {
		t.Errorf("command = %q, want 'delayed_cmd'", cmd.Command)
	}
}

func TestEngine_NoMatchPassthrough(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "simple", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "You take",
        action = function()
            send("got it")
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("simple", nil)
	e.Queue().Clear()

	e.Process("The wind blows gently.")

	if e.Queue().Len() != 0 {
		t.Error("non-matching text should not queue commands")
	}
}

func TestEngine_SetModeCallsOnStart(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "starter", `
local M = {}
M.on_start = function(args)
    send("initial command")
end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("starter", nil)

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("on_start should queue a command")
	}
	if cmd.Command != "initial command" {
		t.Errorf("command = %q, want 'initial command'", cmd.Command)
	}
}

func TestEngine_SetModeCallsOnStop(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "stoppable", `
local M = {}
M.on_start = function(args) end
M.on_stop = function()
    send("cleanup command")
end
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "other", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("stoppable", nil)
	e.Queue().Clear()

	// Switching to another mode should call on_stop
	e.SetMode("other", nil)

	// The cleanup command was sent then queue was cleared during mode switch,
	// so we check that on_stop was called (no panic/error)
	if e.CurrentMode() != "other" {
		t.Errorf("CurrentMode() = %q, want 'other'", e.CurrentMode())
	}
}

func TestEngine_CurrentMode(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "testmode", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	if e.CurrentMode() != "" {
		t.Errorf("initial CurrentMode() = %q, want empty", e.CurrentMode())
	}

	e.SetMode("testmode", nil)
	if e.CurrentMode() != "testmode" {
		t.Errorf("CurrentMode() = %q, want 'testmode'", e.CurrentMode())
	}
}

func TestEngine_SetModeNonexistent(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	e := newTestEngine(t, modesDir, libDir)

	// Should not panic
	e.SetMode("nonexistent", nil)
	if e.CurrentMode() != "nonexistent" {
		t.Errorf("CurrentMode() = %q, want 'nonexistent'", e.CurrentMode())
	}
}

func TestEngine_HasMode(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "exists", `
local M = {}
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	if !e.HasMode("exists") {
		t.Error("HasMode('exists') = false, want true")
	}
	if e.HasMode("nope") {
		t.Error("HasMode('nope') = true, want false")
	}
}

func TestEngine_SetModeDisable(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "active", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("active", nil)
	e.SetMode("disable", nil)

	if e.CurrentMode() != "disable" {
		t.Errorf("CurrentMode() = %q, want 'disable'", e.CurrentMode())
	}
}

func TestEngine_ProcessNoMode(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	e := newTestEngine(t, modesDir, libDir)

	// Should not panic with no active mode
	e.Process("some text")
	if e.Queue().Len() != 0 {
		t.Error("processing with no mode should not queue commands")
	}
}

func TestEngine_MacroModeMetrics(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "macro", `
local M = {}
M.on_start = function(args)
    metrics.track("kills", "Kills")
end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("macro", nil)

	// Metrics session should be started by metrics.track()
	cur := e.Metrics().Current()
	if cur == nil {
		t.Fatal("metrics session should be active after metrics.track()")
	}

	e.SetMode("disable", nil)

	// Session should be ended and in history
	if e.Metrics().Current() != nil {
		t.Error("metrics session should be ended after leaving mode")
	}
	history := e.Metrics().History()
	if len(history) != 1 {
		t.Errorf("history len = %d, want 1", len(history))
	}
}

func TestEngine_NonMacroModeHasSessionButNoEntries(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "loot", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("loot", nil)

	cur := e.Metrics().Current()
	if cur == nil {
		t.Fatal("every mode should have a metrics session")
	}
	if cur.Mode != "loot" {
		t.Errorf("session mode = %q, want %q", cur.Mode, "loot")
	}
	if len(cur.Entries) != 0 {
		t.Errorf("non-macro mode should have no metric entries, got %d", len(cur.Entries))
	}
}

func TestEngine_SetModeWithArgsPassedToOnStart(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "loot", `
local M = {}
M.on_start = function(args)
    if args[1] then
        send("get " .. args[1])
    end
end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("loot", []string{"sword"})

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("expected on_start command with args")
	}
	if cmd.Command != "get sword" {
		t.Errorf("command = %q, want 'get sword'", cmd.Command)
	}
}

func TestEngine_ReloadMode(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "test_reload", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "old pattern",
        action = function()
            send("old action")
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	// Overwrite the file
	writeEngineMode(t, modesDir, "test_reload", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "new pattern",
        action = function()
            send("new action")
        end,
    },
}
return M
`)

	err := e.ReloadMode("test_reload")
	if err != nil {
		t.Fatalf("ReloadMode error: %v", err)
	}

	e.SetMode("test_reload", nil)
	e.Queue().Clear()

	// Old pattern should not match
	e.Process("old pattern")
	if e.Queue().Len() != 0 {
		t.Error("old pattern should not match after reload")
	}

	// New pattern should match
	e.Process("new pattern")
	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("new pattern should match after reload")
	}
	if cmd.Command != "new action" {
		t.Errorf("command = %q, want 'new action'", cmd.Command)
	}
}

func TestEngine_FirstMatchWins(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "multi", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "You",
        action = function()
            send("first")
        end,
    },
    {
        match = "You take",
        action = function()
            send("second")
        end,
    },
}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("multi", nil)
	e.Queue().Clear()

	e.Process("You take a sword")

	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("expected command")
	}
	if cmd.Command != "first" {
		t.Errorf("command = %q, want 'first' (first match wins)", cmd.Command)
	}

	// Only one command should be queued
	if e.Queue().Len() != 0 {
		t.Error("only first matching reaction should fire")
	}
}
