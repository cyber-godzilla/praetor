package engine

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func newTestLuaWithState(t *testing.T) (*lua.LState, *ModeState) {
	t.Helper()
	L := lua.NewState()
	ms := NewModeState()
	RegisterStateAPI(L, ms)
	return L, ms
}

func TestState_SetAndGetRoundTrip(t *testing.T) {
	L, _ := newTestLuaWithState(t)
	defer L.Close()

	err := L.DoString(`
		state.set("count", 42)
		state.set("name", "gladiator")
		state.set("active", true)
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	// Verify from Lua
	err = L.DoString(`
		assert(state.get("count") == 42, "count should be 42")
		assert(state.get("name") == "gladiator", "name should be gladiator")
		assert(state.get("active") == true, "active should be true")
		assert(state.get("nonexistent") == nil, "nonexistent should be nil")
	`)
	if err != nil {
		t.Fatalf("DoString verify error: %v", err)
	}
}

func TestState_SetAndGetFromGo(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	err := L.DoString(`state.set("mykey", "myvalue")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	val, ok := ms.GetValue("mykey")
	if !ok {
		t.Fatal("GetValue('mykey') not found")
	}
	if lua.LVAsString(val) != "myvalue" {
		t.Errorf("GetValue('mykey') = %v, want 'myvalue'", val)
	}
}

func TestState_ReadOnlyMode(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetReadOnly("mode", "macro")

	err := L.DoString(`
		assert(state.mode == "macro", "state.mode should be 'macro', got: " .. tostring(state.mode))
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestState_ReadOnlyCustomKeys(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetReadOnly("region", "arena")
	ms.SetReadOnly("level", 5)

	err := L.DoString(`
		assert(state.region == "arena", "state.region should be 'arena', got: " .. tostring(state.region))
		assert(state.level == 5, "state.level should be 5, got: " .. tostring(state.level))
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestState_Actions(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetActions([]string{"slash", "thrust", "parry"})

	err := L.DoString(`
		local a = state.actions
		assert(a ~= nil, "actions should not be nil")
		assert(#a == 3, "actions length should be 3, got: " .. tostring(#a))
		assert(a[1] == "slash", "actions[1] should be 'slash', got: " .. tostring(a[1]))
		assert(a[2] == "thrust", "actions[2] should be 'thrust', got: " .. tostring(a[2]))
		assert(a[3] == "parry", "actions[3] should be 'parry', got: " .. tostring(a[3]))
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestState_GetActionsFromGo(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetActions([]string{"attack", "defend"})

	actions := ms.GetActions()
	if len(actions) != 2 {
		t.Fatalf("GetActions() len = %d, want 2", len(actions))
	}
	if actions[0] != "attack" || actions[1] != "defend" {
		t.Errorf("GetActions() = %v, want [attack, defend]", actions)
	}
}

func TestState_GetActionsNil(t *testing.T) {
	ms := NewModeState()
	actions := ms.GetActions()
	if actions != nil {
		t.Errorf("GetActions() on new state = %v, want nil", actions)
	}
}

func TestState_ClearResetsUserValues(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	err := L.DoString(`
		state.set("key1", "value1")
		state.set("key2", "value2")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	ms.SetActions([]string{"a", "b"})

	// Verify values exist
	_, ok := ms.GetValue("key1")
	if !ok {
		t.Fatal("key1 should exist before clear")
	}

	ms.Clear()

	// User values should be gone
	_, ok = ms.GetValue("key1")
	if ok {
		t.Error("key1 should not exist after clear")
	}
	_, ok = ms.GetValue("key2")
	if ok {
		t.Error("key2 should not exist after clear")
	}

	// Actions should be nil
	actions := ms.GetActions()
	if actions != nil {
		t.Errorf("GetActions() after clear = %v, want nil", actions)
	}
}

func TestState_ClearPreservesReadOnly(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetReadOnly("mode", "macro")

	err := L.DoString(`state.set("key", "val")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	ms.Clear()

	// Read-only should still be accessible
	err = L.DoString(`
		assert(state.mode == "macro", "state.mode should survive clear")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestState_GetModeState(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	retrieved := GetModeState(L)
	if retrieved != ms {
		t.Error("GetModeState should return the same ModeState instance")
	}
}

func TestState_GoToLua(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected lua.LValue
	}{
		{"hello", lua.LString("hello")},
		{42, lua.LNumber(42)},
		{3.14, lua.LNumber(3.14)},
		{true, lua.LBool(true)},
		{false, lua.LBool(false)},
		{nil, lua.LNil},
	}

	for _, tt := range tests {
		got := goToLua(tt.input)
		if got != tt.expected {
			t.Errorf("goToLua(%v) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestState_PersistSurvivesClear(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetPersist("armor")
	err := L.DoString(`
		state.set("armor", "test_value")
		state.set("temp", "gone")
	`)
	if err != nil {
		t.Fatal(err)
	}

	ms.Clear()

	err = L.DoString(`
		assert(state.get("armor") == "test_value", "persistent key should survive clear, got: " .. tostring(state.get("armor")))
		assert(state.get("temp") == nil, "non-persistent key should be cleared")
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestState_PersistentSnapshot(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	ms.SetPersist("kills")
	L.DoString(`state.set("kills", 42)`)
	L.DoString(`state.set("temp", "not persisted")`)

	snap := ms.PersistentSnapshot()
	if snap["kills"] != float64(42) {
		t.Errorf("kills = %v, want 42", snap["kills"])
	}
	if _, ok := snap["temp"]; ok {
		t.Error("temp should not be in snapshot")
	}
}

func TestState_PersistFromLua(t *testing.T) {
	L, ms := newTestLuaWithState(t)
	defer L.Close()

	err := L.DoString(`
		state.persist("my_key")
		state.set("my_key", "hello")
	`)
	if err != nil {
		t.Fatal(err)
	}

	if !ms.IsPersistent("my_key") {
		t.Error("my_key should be persistent")
	}

	snap := ms.PersistentSnapshot()
	if snap["my_key"] != "hello" {
		t.Errorf("my_key = %v, want 'hello'", snap["my_key"])
	}
}

func TestState_NonexistentReadOnlyReturnsNil(t *testing.T) {
	L, _ := newTestLuaWithState(t)
	defer L.Close()

	err := L.DoString(`
		assert(state.nonexistent_field == nil, "nonexistent read-only field should be nil")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}
