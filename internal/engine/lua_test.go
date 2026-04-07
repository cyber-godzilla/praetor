package engine

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func setupTestDirs(t *testing.T) (modesDir, libDir string) {
	t.Helper()
	modesDir = t.TempDir()
	libDir = t.TempDir()
	return modesDir, libDir
}

func writeMode(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name+".lua"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing mode file %s: %v", name, err)
	}
}

func TestLuaVM_LoadMinimalMode(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	writeMode(t, modesDir, "loot", `
local M = {}

M.on_start = function(args)
    send("look")
end

M.reactions = {
    {
        match = "You take",
        action = function()
            send("get item")
        end,
    },
}

return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	// Register a stub send() so the Lua file can reference it
	vm.State().SetGlobal("send", vm.State().NewFunction(func(L *lua.LState) int {
		return 0
	}))

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, ok := vm.GetMode("loot")
	if !ok {
		t.Fatal("GetMode('loot') not found")
	}
	if mode.Name != "loot" {
		t.Errorf("mode.Name = %q, want 'loot'", mode.Name)
	}
	if mode.onStartRef == nil {
		t.Error("mode.onStartRef should not be nil")
	}
	if mode.HasOnStop {
		t.Error("mode.HasOnStop should be false")
	}
	if len(mode.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(mode.Reactions))
	}

	r := mode.Reactions[0]
	if len(r.RawPatterns) != 1 || r.RawPatterns[0] != "You take" {
		t.Errorf("RawPatterns = %v, want ['You take']", r.RawPatterns)
	}
	if r.actionRef == nil {
		t.Error("actionRef should not be nil")
	}
	if r.HasCondition {
		t.Error("HasCondition should be false")
	}
	if r.DelayMs != 0 {
		t.Errorf("DelayMs = %d, want 0", r.DelayMs)
	}
}

func TestLuaVM_LoadModeWithOnStop(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	writeMode(t, modesDir, "macro", `
local M = {}

M.on_start = function(args)
end

M.on_stop = function()
end

M.reactions = {}

return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, ok := vm.GetMode("macro")
	if !ok {
		t.Fatal("GetMode('macro') not found")
	}
	if !mode.HasOnStop {
		t.Error("mode.HasOnStop should be true")
	}
	if mode.onStopRef == nil {
		t.Error("mode.onStopRef should not be nil")
	}
}

func TestLuaVM_LoadModeWithConditionAndDelay(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	writeMode(t, modesDir, "combat", `
local M = {}

M.on_start = function(args)
end

M.reactions = {
    {
        match = {"You attack *", "You strike *"},
        action = function()
        end,
        condition = function()
            return true
        end,
        delay = 750,
    },
}

return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, ok := vm.GetMode("combat")
	if !ok {
		t.Fatal("GetMode('combat') not found")
	}
	if len(mode.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(mode.Reactions))
	}

	r := mode.Reactions[0]
	if len(r.RawPatterns) != 2 {
		t.Fatalf("len(RawPatterns) = %d, want 2", len(r.RawPatterns))
	}
	if r.RawPatterns[0] != "You attack *" || r.RawPatterns[1] != "You strike *" {
		t.Errorf("RawPatterns = %v", r.RawPatterns)
	}
	if !r.HasCondition {
		t.Error("HasCondition should be true")
	}
	if r.conditionRef == nil {
		t.Error("conditionRef should not be nil")
	}
	if r.DelayMs != 750 {
		t.Errorf("DelayMs = %d, want 750", r.DelayMs)
	}
	// Verify patterns are compiled (wildcard patterns)
	if len(r.Patterns) != 2 {
		t.Fatalf("len(Patterns) = %d, want 2", len(r.Patterns))
	}
}

func TestLuaVM_HotReload(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	// Initial version
	writeMode(t, modesDir, "loot", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "You take",
        action = function() end,
    },
}
return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, _ := vm.GetMode("loot")
	if len(mode.Reactions) != 1 {
		t.Fatalf("initial: len(Reactions) = %d, want 1", len(mode.Reactions))
	}
	if mode.Reactions[0].RawPatterns[0] != "You take" {
		t.Errorf("initial pattern = %q, want 'You take'", mode.Reactions[0].RawPatterns[0])
	}

	// Overwrite with new version
	writeMode(t, modesDir, "loot", `
local M = {}
M.on_start = function(args) end
M.reactions = {
    {
        match = "You grab",
        action = function() end,
    },
    {
        match = "You drop",
        action = function() end,
    },
}
return M
`)

	err = vm.ReloadMode("loot")
	if err != nil {
		t.Fatalf("ReloadMode() error: %v", err)
	}

	mode, _ = vm.GetMode("loot")
	if len(mode.Reactions) != 2 {
		t.Fatalf("reloaded: len(Reactions) = %d, want 2", len(mode.Reactions))
	}
	if mode.Reactions[0].RawPatterns[0] != "You grab" {
		t.Errorf("reloaded pattern[0] = %q, want 'You grab'", mode.Reactions[0].RawPatterns[0])
	}
	if mode.Reactions[1].RawPatterns[0] != "You drop" {
		t.Errorf("reloaded pattern[1] = %q, want 'You drop'", mode.Reactions[1].RawPatterns[0])
	}
}

func TestLuaVM_ModeNames(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	writeMode(t, modesDir, "alpha", `
local M = {}
M.on_start = function() end
M.reactions = {}
return M
`)
	writeMode(t, modesDir, "beta", `
local M = {}
M.on_start = function() end
M.reactions = {}
return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	names := vm.ModeNames()
	if len(names) != 2 {
		t.Fatalf("len(ModeNames()) = %d, want 2", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("ModeNames() = %v, want [alpha, beta]", names)
	}
}

func TestLuaVM_GetModeNotFound(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	_, ok := vm.GetMode("nonexistent")
	if ok {
		t.Error("GetMode('nonexistent') should return false")
	}
}

func TestLuaVM_LoadModesEmptyDir(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() on empty dir error: %v", err)
	}

	names := vm.ModeNames()
	if len(names) != 0 {
		t.Errorf("ModeNames() = %v, want empty", names)
	}
}

func TestLuaVM_LibDirRequire(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	// Write a shared lib file
	err := os.WriteFile(filepath.Join(libDir, "helpers.lua"), []byte(`
local H = {}
H.greeting = "hello"
return H
`), 0644)
	if err != nil {
		t.Fatalf("writing lib file: %v", err)
	}

	writeMode(t, modesDir, "test_require", `
local helpers = require("helpers")
local M = {}
M.on_start = function() end
M.reactions = {
    {
        match = helpers.greeting,
        action = function() end,
    },
}
return M
`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err = vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, ok := vm.GetMode("test_require")
	if !ok {
		t.Fatal("GetMode('test_require') not found")
	}
	if len(mode.Reactions) != 1 {
		t.Fatalf("len(Reactions) = %d, want 1", len(mode.Reactions))
	}
	if mode.Reactions[0].RawPatterns[0] != "hello" {
		t.Errorf("pattern = %q, want 'hello'", mode.Reactions[0].RawPatterns[0])
	}
}

func TestLuaVM_InvalidModeFileSkipped(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	writeMode(t, modesDir, "bad", `this is not valid lua!!!`)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() should skip invalid files, got error: %v", err)
	}

	// The bad mode should not be loaded.
	_, ok := vm.GetMode("bad")
	if ok {
		t.Error("invalid mode 'bad' should not be loaded")
	}
}

func TestLuaVM_Matcher(t *testing.T) {
	modesDir, libDir := setupTestDirs(t)

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	m := vm.Matcher()
	if m == nil {
		t.Fatal("Matcher() should not be nil")
	}

	// Verify it works
	cp := m.Compile("hello")
	if !m.Match(cp, "hello world") {
		t.Error("Matcher should match substring")
	}
}

func TestLoadAllModes(t *testing.T) {
	modesDir := filepath.Join("..", "..", "..", "configs", "modes")
	libDir := filepath.Join("..", "..", "..", "configs", "lib")

	// Verify the directories exist
	if _, err := os.Stat(modesDir); os.IsNotExist(err) {
		t.Skipf("modes directory not found at %s", modesDir)
	}
	if _, err := os.Stat(libDir); os.IsNotExist(err) {
		t.Skipf("lib directory not found at %s", libDir)
	}

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	// Register stub globals that modes may call at require/load time
	L := vm.State()
	noop := L.NewFunction(func(L *lua.LState) int { return 0 })
	L.SetGlobal("send", noop)
	L.SetGlobal("set_mode", noop)
	L.SetGlobal("notify", noop)
	L.SetGlobal("log", noop)
	L.SetGlobal("random_item", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		if tbl.Len() > 0 {
			L.Push(tbl.RawGetInt(1))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))

	// time table
	timeTbl := L.NewTable()
	timeTbl.RawSetString("now", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))
	timeTbl.RawSetString("since", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))
	L.SetGlobal("time", timeTbl)

	// state table with get/set
	stateTbl := L.NewTable()
	stateVals := make(map[string]lua.LValue)
	stateTbl.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		if v, ok := stateVals[key]; ok {
			L.Push(v)
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	stateTbl.RawSetString("set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := L.Get(2)
		stateVals[key] = val
		return 0
	}))
	// metatable for read-only fields
	mt := L.NewTable()
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNil)
		return 1
	}))
	L.SetMetatable(stateTbl, mt)
	L.SetGlobal("state", stateTbl)

	// metrics table
	metricsTbl := L.NewTable()
	metricsTbl.RawSetString("track", noop)
	metricsTbl.RawSetString("inc", noop)
	metricsTbl.RawSetString("dec", noop)
	metricsTbl.RawSetString("set", noop)
	metricsTbl.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(0))
		return 1
	}))
	L.SetGlobal("metrics", metricsTbl)

	// status table
	statusTbl := L.NewTable()
	statusMT := L.NewTable()
	statusMT.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(100))
		return 1
	}))
	L.SetMetatable(statusTbl, statusMT)
	L.SetGlobal("status", statusTbl)

	err := vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	names := vm.ModeNames()
	t.Logf("Loaded %d modes: %v", len(names), names)

	if len(names) < 20 {
		t.Errorf("expected at least 20 modes, got %d", len(names))
	}

	// Verify each mode has valid structure
	for _, name := range names {
		mode, ok := vm.GetMode(name)
		if !ok {
			t.Errorf("GetMode(%q) returned false", name)
			continue
		}
		if mode.Name != name {
			t.Errorf("mode.Name = %q, want %q", mode.Name, name)
		}
		t.Logf("  %s: %d reactions, has_on_start=%v, has_on_stop=%v",
			name, len(mode.Reactions), mode.onStartRef != nil, mode.HasOnStop)
	}
}
