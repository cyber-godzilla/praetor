package engine

import (
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// withShortScriptTimeout shrinks the Lua execution deadline for the duration of
// a test so infinite-loop cases abort quickly.
func withShortScriptTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := scriptTimeout
	scriptTimeout = d
	t.Cleanup(func() { scriptTimeout = prev })
}

func TestEngine_ReactionSetModeThenSend_NotDropped(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "trigger", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    { match = "go", action = function() set_mode("target"); send("after") end },
}
return M
`)
	writeEngineMode(t, modesDir, "target", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)
	e.SetMode("trigger", nil)

	// A reaction that switches mode and then sends: the set_mode nests a Lua call
	// (target's on_start via callLua). A non-nesting-safe deadline wrapper nils the
	// outer Lua context and panics, so send("after") is silently dropped.
	e.Process("go now")

	if e.CurrentMode() != "target" {
		t.Fatalf("CurrentMode = %q, want target", e.CurrentMode())
	}
	found := false
	for {
		cmd, ok := e.Queue().Dequeue()
		if !ok {
			break
		}
		if cmd.Command == "after" {
			found = true
		}
	}
	if !found {
		t.Fatal(`send("after") after set_mode was dropped — nested callLua nils the outer Lua context`)
	}
}

func TestEngine_TimerSetMode_NotDropped(t *testing.T) {
	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "tmr", `
local M = {}
M.on_start = function(args)
    set_timeout(function() set_mode("dest"); send("post") end, 10)
end
M.reactions = {}
return M
`)
	writeEngineMode(t, modesDir, "dest", `
local M = {}
M.on_start = function(args) end
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)
	e.SetMode("tmr", nil)

	time.Sleep(150 * time.Millisecond) // let the timer fire

	if e.CurrentMode() != "dest" {
		t.Fatalf("CurrentMode = %q, want dest", e.CurrentMode())
	}
	found := false
	for {
		cmd, ok := e.Queue().Dequeue()
		if !ok {
			break
		}
		if cmd.Command == "post" {
			found = true
		}
	}
	if !found {
		t.Fatal(`send("post") after set_mode in a timer callback was dropped`)
	}
}

func TestEngine_InfiniteLoopReactionAbortsWithinDeadline(t *testing.T) {
	withShortScriptTimeout(t, 200*time.Millisecond)

	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "hang", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    { match = "trigger", action = function() while true do end end },
}
return M
`)
	e := newTestEngine(t, modesDir, libDir)
	e.SetMode("hang", nil)

	done := make(chan struct{})
	go func() {
		e.Process("trigger this")
		close(done)
	}()

	select {
	case <-done:
		// aborted by the deadline
	case <-time.After(3 * time.Second):
		t.Fatal("Process never returned; the infinite loop was not bounded by a deadline")
	}

	// The engine mutex was released and the VM is still usable: the next line
	// processes normally.
	e.Process("harmless line")
}

func TestEngine_InfiniteLoopOnStartAbortsWithinDeadline(t *testing.T) {
	withShortScriptTimeout(t, 200*time.Millisecond)

	modesDir, libDir := setupEngineTestDirs(t)
	writeEngineMode(t, modesDir, "hangstart", `
local M = {}
M.on_start = function(args) while true do end end
M.reactions = {}
return M
`)
	e := newTestEngine(t, modesDir, libDir)

	done := make(chan struct{})
	go func() {
		e.SetMode("hangstart", nil)
		close(done)
	}()

	select {
	case <-done:
		// aborted
	case <-time.After(3 * time.Second):
		t.Fatal("SetMode never returned; on_start infinite loop was not bounded")
	}
}

func TestTimerManager_InfiniteLoopCallbackAbortsWithinDeadline(t *testing.T) {
	withShortScriptTimeout(t, 200*time.Millisecond)

	tm, L, mu := newTestTimerManager(t)

	mu.Lock()
	cb := L.NewFunction(func(*lua.LState) int {
		L.DoString("while true do end")
		return 0
	})
	mu.Unlock()

	tm.SetTimeout(cb, 10)

	// The callback runs under luaMu; if it isn't bounded, this Lock never returns.
	done := make(chan struct{})
	go func() {
		mu.Lock()
		mu.Unlock()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timer callback wedged the Lua mutex; deadline did not bound it")
	}
}
