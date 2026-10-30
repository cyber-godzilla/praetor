package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSandboxLua(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictedSourceHasSafeAPIsWithoutDangerousStandardLibraries(t *testing.T) {
	dir := t.TempDir()
	writeSandboxLua(t, dir, "lib_value.lua", `return { value = "sandboxed" }`)
	writeSandboxLua(t, dir, "safe.lua", `
assert(os == nil and io == nil and debug == nil and package == nil)
assert(dofile == nil and loadfile == nil and load == nil and loadstring == nil)
assert(getfenv == nil and setfenv == nil and collectgarbage == nil and channel == nil)
assert(type(table) == "table" and type(string) == "table" and type(math) == "table")
assert(type(coroutine) == "table" and type(require) == "function")
local lib = require("lib_value")
return { desc = lib.value, on_start = function() end, reactions = {} }
`)
	vm := NewLuaVMWithSources([]LuaSource{{Path: dir, Trust: LuaTrustRestricted}})
	t.Cleanup(vm.Close)
	if err := vm.LoadModes(); err != nil {
		t.Fatal(err)
	}
	mode, ok := vm.GetMode("safe")
	if !ok || mode.Desc != "sandboxed" {
		t.Fatalf("restricted safe mode = %+v, %v", mode, ok)
	}
}

func TestRestrictedRequireCannotEscapeOrCrossSources(t *testing.T) {
	parent := t.TempDir()
	restricted := filepath.Join(parent, "restricted")
	trusted := filepath.Join(parent, "trusted")
	writeSandboxLua(t, trusted, "secret.lua", `return { value = "secret" }`)
	writeSandboxLua(t, restricted, "cross.lua", `local x = require("secret"); return { desc = x.value }`)
	writeSandboxLua(t, restricted, "traverse.lua", `local x = require("../trusted/secret"); return { desc = x.value }`)
	vm := NewLuaVMWithSources([]LuaSource{
		{Path: trusted, Trust: LuaTrustTrusted},
		{Path: restricted, Trust: LuaTrustRestricted},
	})
	t.Cleanup(vm.Close)
	if err := vm.LoadModes(); err != nil {
		t.Fatal(err)
	}
	if _, ok := vm.GetMode("cross"); ok {
		t.Fatal("restricted source imported a trusted-source module")
	}
	if _, ok := vm.GetMode("traverse"); ok {
		t.Fatal("restricted source traversed outside its root")
	}
}

func TestRestrictedRequireRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	restricted := filepath.Join(parent, "restricted")
	outside := filepath.Join(parent, "outside.lua")
	writeSandboxLua(t, parent, "outside.lua", `return { value = "escaped" }`)
	if err := os.MkdirAll(restricted, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(restricted, "linked.lua")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeSandboxLua(t, restricted, "escape.lua", `local x = require("linked"); return { desc = x.value }`)

	vm := NewLuaVMWithSources([]LuaSource{{Path: restricted, Trust: LuaTrustRestricted}})
	t.Cleanup(vm.Close)
	if err := vm.LoadModes(); err != nil {
		t.Fatal(err)
	}
	if _, ok := vm.GetMode("escape"); ok {
		t.Fatal("restricted source imported a symlink target outside its root")
	}
	if _, ok := vm.GetMode("linked"); ok {
		t.Fatal("restricted source loaded an outside symlink as a mode")
	}
}

func TestTrustedCapabilitiesCannotLeakIntoRestrictedLibraries(t *testing.T) {
	trusted := t.TempDir()
	restricted := t.TempDir()
	writeSandboxLua(t, trusted, "mutate.lua", `
string.launch_process = os.execute
return { on_start = function() end, reactions = {} }
`)
	writeSandboxLua(t, restricted, "isolated.lua", `
assert(string.launch_process == nil)
return { on_start = function() end, reactions = {} }
`)
	vm := NewLuaVMWithSources([]LuaSource{
		{Path: trusted, Trust: LuaTrustTrusted},
		{Path: restricted, Trust: LuaTrustRestricted},
	})
	t.Cleanup(vm.Close)
	if err := vm.LoadModes(); err != nil {
		t.Fatal(err)
	}
	if _, ok := vm.GetMode("isolated"); !ok {
		t.Fatal("trusted global mutation leaked into restricted source")
	}
}

func TestTrustedSourceRetainsLegacyStandardLibraryAccess(t *testing.T) {
	dir := t.TempDir()
	writeSandboxLua(t, dir, "legacy.lua", `
assert(type(os.getenv) == "function" and type(io.open) == "function")
return { on_start = function() end, reactions = {} }
`)
	vm := NewLuaVMWithSources([]LuaSource{{Path: dir, Trust: LuaTrustTrusted}})
	t.Cleanup(vm.Close)
	if err := vm.LoadModes(); err != nil {
		t.Fatal(err)
	}
	if _, ok := vm.GetMode("legacy"); !ok {
		t.Fatal("trusted source lost legacy standard-library access")
	}
}

func TestRestrictedModeUsesPraetorAPIs(t *testing.T) {
	dir := t.TempDir()
	writeSandboxLua(t, dir, "safe.lua", `
return {
  on_start = function()
    state.set("nested", { value = 7 })
    metrics.track("starts", "Starts")
    metrics.inc("starts")
    set_timeout(function() end, 1000)
    send("sandbox command")
  end,
  reactions = {}
}
`)
	e, err := NewEngineWithSources([]LuaSource{{Path: dir, Trust: LuaTrustRestricted}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	e.SetMode("safe", nil)
	command, ok := e.Queue().Dequeue()
	if !ok || command.Command != "sandbox command" {
		t.Fatalf("restricted send result = %+v, %v", command, ok)
	}
	if got := e.Metrics().Get("starts"); got != 1 {
		t.Fatalf("restricted metric = %d, want 1", got)
	}
}

func TestTrustedLuaValuesCannotCrossSharedStateIntoRestrictedMode(t *testing.T) {
	trusted := t.TempDir()
	restricted := t.TempDir()
	writeSandboxLua(t, trusted, "trusted.lua", `
return {
  on_start = function()
    state.set("capability", os.execute)
    state.set("nested_capability", { launch = os.execute })
    state.persist("capability")
    state.persist("nested_capability")
  end,
  reactions = {}
}
`)
	writeSandboxLua(t, restricted, "restricted.lua", `
return {
  on_start = function()
    assert(state.get("capability") == nil)
    assert(state.get("nested_capability") == nil)
    send("contained")
  end,
  reactions = {}
}
`)
	e, err := NewEngineWithSources([]LuaSource{
		{Path: trusted, Trust: LuaTrustTrusted},
		{Path: restricted, Trust: LuaTrustRestricted},
	}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	e.SetMode("trusted", nil)
	e.SetMode("restricted", nil)
	command, ok := e.Queue().Dequeue()
	if !ok || command.Command != "contained" {
		t.Fatalf("restricted state containment result = %+v, %v", command, ok)
	}
}

func TestRuntimeRebuildRehomesLuaStateValuesBeforeClosingOldVM(t *testing.T) {
	dir := t.TempDir()
	writeSandboxLua(t, dir, "safe.lua", `
return {
  on_start = function()
    state.set("record", { count = 3 })
    state.persist("record")
  end,
  reactions = {}
}
`)
	sources := []LuaSource{{Path: dir, Trust: LuaTrustRestricted}}
	e, err := NewEngineWithSources(sources, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	e.SetMode("safe", nil)
	oldOwner := e.state.valueOwners["record"]
	if oldOwner == nil {
		t.Fatal("test value has no owning Lua state")
	}
	if err := e.UpdateScriptSources(sources); err != nil {
		t.Fatal(err)
	}
	newOwner := e.state.valueOwners["record"]
	if newOwner == nil || newOwner == oldOwner {
		t.Fatal("state value remained owned by the retired Lua VM")
	}
	if got := e.state.PersistentSnapshot()["record"]; got == nil {
		t.Fatal("portable persistent value was lost during runtime rebuild")
	}
}
