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

	if e.CurrentMode() != "other" {
		t.Errorf("CurrentMode() = %q, want 'other'", e.CurrentMode())
	}

	// on_stop's cleanup send must survive the switch: the outgoing mode's pending
	// queue is cleared *before* on_stop runs, so on_stop's own sends (sheathe,
	// stand, ...) reach the wire instead of being wiped immediately after.
	cmd, ok := e.Queue().Dequeue()
	if !ok {
		t.Fatal("on_stop's cleanup command did not survive the mode switch")
	}
	if cmd.Command != "cleanup command" {
		t.Errorf("surviving command = %q, want 'cleanup command'", cmd.Command)
	}
}

func TestEngine_CloseFlushesPersistentStateSynchronously(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "saver", `
local M = {}
M.on_start = function(args)
    state.persist("token")
    state.set("token", "kept")
end
M.reactions = {}
return M
`)
	dataDir := t.TempDir()

	e, err := NewEngine([]string{modesDir, libDir}, nil, dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.SetUsername("alice")
	e.SetMode("saver", nil) // on_start persists token=kept and marks dirty

	// Close must flush synchronously — the 5s debounce would otherwise lose this
	// on quit.
	e.Close()

	e2, err := NewEngine([]string{modesDir, libDir}, nil, dataDir)
	if err != nil {
		t.Fatalf("NewEngine 2: %v", err)
	}
	defer e2.Close()
	e2.SetUsername("alice")

	v, ok := e2.State().GetValue("token")
	if !ok {
		t.Fatal("persistent key not loaded after Close flush (state lost on quit)")
	}
	if v.String() != "kept" {
		t.Errorf("loaded persistent value = %q, want kept", v.String())
	}
}

func TestEngine_SetMode_DeferredRecursionLandsOnTarget(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "a", `
local M = {}
M.on_start = function(args) set_mode("b") end
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "b", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("a", nil)

	if e.CurrentMode() != "b" {
		t.Fatalf("CurrentMode = %q, want b", e.CurrentMode())
	}
	// One deferred hop — not a deep recursion churning many metric sessions.
	if n := len(e.Metrics().History()); n > 3 {
		t.Fatalf("metrics history churned %d sessions for a single deferred switch", n)
	}
}

func TestEngine_SetMode_PingPongCappedAndUsable(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "a", `
local M = {}
M.on_start = function(args) set_mode("b") end
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "b", `
local M = {}
M.on_start = function(args) set_mode("a") end
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("a", nil) // a↔b ping-pong: must cap, not recurse ~128 deep

	// The loop is bounded, so the metric-session churn is small (well under the
	// old ~128 / the 50-entry history cap).
	if n := len(e.Metrics().History()); n > 12 {
		t.Fatalf("mode-switch loop churned %d sessions; expected it to be capped", n)
	}
	// The engine remains usable after the capped loop.
	e.SetMode("disable", nil)
	if e.CurrentMode() != "disable" {
		t.Fatalf("engine unusable after a capped switch loop: mode = %q", e.CurrentMode())
	}
}

func TestEngine_ModeChanges_LatestWinsUnderFlood(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "alpha", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "beta", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	// Flood past the mode-change channel's buffer with no reader draining, then
	// switch to the final mode. The consumer only needs the latest state, so the
	// final switch must not be the one that gets dropped.
	for i := 0; i < 16; i++ {
		e.SetMode("alpha", nil)
	}
	e.SetMode("beta", nil)

	var last string
	for {
		select {
		case mc := <-e.ModeChanges():
			last = mc.NewMode
			continue
		default:
		}
		break
	}
	if last != "beta" {
		t.Fatalf("last mode change seen = %q, want beta (final switch was dropped under flood)", last)
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

func TestEngine_ModeNamesExcludesLibPrefix(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)

	writeEngineMode(t, modesDir, "alpha", `
local M = {}
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "lib_util", `
local M = {}
M.reactions = {}
return M
`)

	e := newTestEngine(t, modesDir, libDir)

	names := e.ModeNames()
	for _, n := range names {
		if n == "lib_util" {
			t.Errorf("ModeNames() = %v, should exclude lib_-prefixed modes", names)
		}
	}
	found := false
	for _, n := range names {
		if n == "alpha" {
			found = true
		}
	}
	if !found {
		t.Errorf("ModeNames() = %v, want to include 'alpha'", names)
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

func TestVM_ResolveModeName(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "Fishing", `
local M = {}
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	if got, ok := e.vm.ResolveModeName("fishing"); !ok || got != "Fishing" {
		t.Errorf("ResolveModeName(\"fishing\") = (%q, %v), want (\"Fishing\", true)", got, ok)
	}
	if got, ok := e.vm.ResolveModeName("Fishing"); !ok || got != "Fishing" {
		t.Errorf("ResolveModeName(\"Fishing\") = (%q, %v), want (\"Fishing\", true)", got, ok)
	}
	if _, ok := e.vm.ResolveModeName("nope"); ok {
		t.Error("ResolveModeName(\"nope\") ok = true, want false")
	}
}

func TestEngine_HasModeCaseInsensitive(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "Fishing", `
local M = {}
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	if !e.HasMode("fishing") {
		t.Error("HasMode(\"fishing\") = false, want true")
	}
}

func TestEngine_SetModeCanonicalizesName(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "Fishing", `
local M = {}
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	e.SetMode("fishing", nil)
	if e.CurrentMode() != "Fishing" {
		t.Errorf("CurrentMode() = %q, want canonical \"Fishing\"", e.CurrentMode())
	}
}

func TestEngineNotificationSink(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	e := newTestEngine(t, modesDir, libDir)
	got := make(chan [2]string, 1)
	e.SetNotificationSink(func(title, message string) {
		got <- [2]string{title, message}
	})
	e.OnNotify("Alert", "A test notification")

	select {
	case notification := <-got:
		if notification != [2]string{"Alert", "A test notification"} {
			t.Fatalf("notification = %#v", notification)
		}
	case <-time.After(time.Second):
		t.Fatal("notification sink was not called")
	}
}
