package engine

import (
	"sync"
	"testing"
)

// TestEngine_PersistFlushRaceWithProcess is a regression guard (run with -race):
// the debounced persistence flush ran its snapshot on a timer goroutine without
// the engine mutex, iterating live Lua tables (LTable.ForEach) while a reaction
// mutated the same table under e.mu — "fatal error: concurrent map iteration
// and map write". The armor-tracker shape (persist a table, keep mutating it in
// reactions) triggers it. The snapshot must now serialize with Process.
func TestEngine_PersistFlushRaceWithProcess(t *testing.T) {
	modesDir, _ := setupEngineTestDirs(t)
	dataDir := t.TempDir()
	writeEngineMode(t, modesDir, "tracker", `
local M = {}
M.on_start = function()
    state.persist("t")
    state.set("t", {})
end
M.reactions = {
    {
        match = "tick",
        action = function()
            local t = state.get("t")
            t.n = (t.n or 0) + 1
            t["k" .. tostring(t.n % 8)] = t.n
            state.set("t", t) -- re-mark dirty so the flush keeps snapshotting
        end,
    },
}
return M
`)
	e, err := NewEngine([]string{modesDir}, nil, dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer e.Close()
	e.SetUsername("racer")
	e.SetMode("tracker", nil)

	store := e.PersistentStore()
	if store == nil {
		t.Fatal("expected a persistent store after SetUsername")
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 3000; i++ {
			e.Process("tick")
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 3000; i++ {
			store.Flush()
		}
	}()
	wg.Wait()
}

// TestEngine_PersistFlushWritesData confirms the now-locked snapshot path still
// produces correct data on disk (the fix must not break the flush itself).
func TestEngine_PersistFlushWritesData(t *testing.T) {
	modesDir, _ := setupEngineTestDirs(t)
	dataDir := t.TempDir()
	writeEngineMode(t, modesDir, "counter", `
local M = {}
M.on_start = function()
    state.persist("kills")
    state.set("kills", 7)
end
return M
`)
	e, err := NewEngine([]string{modesDir}, nil, dataDir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	e.SetUsername("hero")
	e.SetMode("counter", nil)
	e.PersistentStore().Flush()
	e.Close()

	// Reload from disk with a fresh store.
	data, err := NewPersistentStore(dataDir, "hero").Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if data["kills"] != float64(7) {
		t.Errorf("persisted kills = %v, want 7", data["kills"])
	}
}
