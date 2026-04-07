package engine

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// modeStateKey is the Lua registry key for the ModeState userdata.
const modeStateKey = "__mode_state__"

// DisplayItem maps a user-facing label to a state key for sidebar display
// and /toggle, /set commands.
type DisplayItem struct {
	Key   string
	Label string
}

// ModeState holds user-defined and read-only state accessible from Lua.
type ModeState struct {
	mu             sync.RWMutex
	values         map[string]lua.LValue
	readOnly       map[string]lua.LValue
	persistentKeys map[string]bool // keys that survive Clear()
	onPersistDirty func()          // called when a persistent key changes
	displayItems   []DisplayItem   // label→key mappings for sidebar/commands
	actions        *lua.LTable
	luaState       *lua.LState // reference for creating tables
}

// NewModeState creates a new empty ModeState.
func NewModeState() *ModeState {
	return &ModeState{
		values:         make(map[string]lua.LValue),
		readOnly:       make(map[string]lua.LValue),
		persistentKeys: make(map[string]bool),
	}
}

// Clear resets all user-defined values. Read-only fields are not affected.
// Persistent keys are preserved across clears.
func (ms *ModeState) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	saved := make(map[string]lua.LValue)
	for key := range ms.persistentKeys {
		if val, ok := ms.values[key]; ok {
			saved[key] = val
		}
	}

	ms.values = make(map[string]lua.LValue)
	for key, val := range saved {
		ms.values[key] = val
	}

	ms.displayItems = nil
	ms.actions = nil
}

// AddDisplay registers a state key for sidebar display with a user-facing label.
func (ms *ModeState) AddDisplay(key, label string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	// Update existing if key already declared.
	for i, d := range ms.displayItems {
		if d.Key == key {
			ms.displayItems[i].Label = label
			return
		}
	}
	ms.displayItems = append(ms.displayItems, DisplayItem{Key: key, Label: label})
}

// DisplayItems returns the current display declarations.
func (ms *ModeState) DisplayItems() []DisplayItem {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	cp := make([]DisplayItem, len(ms.displayItems))
	copy(cp, ms.displayItems)
	return cp
}

// DisplayValues returns label→value pairs for sidebar rendering.
func (ms *ModeState) DisplayValues() []struct{ Label, Value string } {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var result []struct{ Label, Value string }
	for _, d := range ms.displayItems {
		val, ok := ms.values[d.Key]
		valStr := "nil"
		if ok {
			valStr = luaToString(val)
		}
		result = append(result, struct{ Label, Value string }{d.Label, valStr})
	}
	return result
}

// ResolveLabel finds the state key for a display label (case-insensitive).
func (ms *ModeState) ResolveLabel(label string) (string, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	lower := strings.ToLower(label)
	for _, d := range ms.displayItems {
		if strings.ToLower(d.Label) == lower {
			return d.Key, true
		}
	}
	return "", false
}

// Toggle inverts a boolean state value by key.
func (ms *ModeState) Toggle(key string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	val, ok := ms.values[key]
	if !ok {
		ms.values[key] = lua.LTrue
		return
	}
	if val == lua.LTrue {
		ms.values[key] = lua.LFalse
	} else {
		ms.values[key] = lua.LTrue
	}
}

// SetFromString sets a state value by key, parsing the string as number, bool, or string.
func (ms *ModeState) SetFromString(key, value string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.values[key] = parseStringToLua(value)
}

func luaToString(v lua.LValue) string {
	switch val := v.(type) {
	case lua.LBool:
		if val {
			return "true"
		}
		return "false"
	case lua.LNumber:
		s := val.String()
		return s
	case *lua.LNilType:
		return "nil"
	default:
		return v.String()
	}
}

func parseStringToLua(s string) lua.LValue {
	lower := strings.ToLower(s)
	if lower == "true" {
		return lua.LTrue
	}
	if lower == "false" {
		return lua.LFalse
	}
	if lower == "nil" || lower == "null" {
		return lua.LNil
	}
	// Try number.
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return lua.LNumber(n)
	}
	return lua.LString(s)
}

// GetReadOnlyInt returns a read-only integer value, or the default if not found.
func (ms *ModeState) GetReadOnlyInt(key string, defaultVal int) int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.readOnly[key]
	if !ok {
		return defaultVal
	}
	if n, ok := val.(lua.LNumber); ok {
		return int(n)
	}
	return defaultVal
}

// SetReadOnly sets a read-only field accessible as state.<key> in Lua.
// Accepts Go string, int, float64, bool, or nil.
func (ms *ModeState) SetReadOnly(key string, value interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.readOnly[key] = goToLua(value)
}

// SetActions sets the actions array (exposed as state.actions in Lua).
func (ms *ModeState) SetActions(actions []string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.luaState == nil {
		return
	}

	tbl := ms.luaState.NewTable()
	for i, a := range actions {
		tbl.RawSetInt(i+1, lua.LString(a))
	}
	ms.actions = tbl
}

// GetActions returns the current actions as a Go string slice.
func (ms *ModeState) GetActions() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.actions == nil {
		return nil
	}

	var result []string
	ms.actions.ForEach(func(_, value lua.LValue) {
		if s, ok := value.(lua.LString); ok {
			result = append(result, string(s))
		}
	})
	return result
}

// GetValue returns a user-defined value by key.
func (ms *ModeState) GetValue(key string) (lua.LValue, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	v, ok := ms.values[key]
	return v, ok
}

// SetPersist marks a key as persistent so it survives Clear().
func (ms *ModeState) SetPersist(key string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.persistentKeys[key] = true
}

// IsPersistent returns whether a key is marked as persistent.
func (ms *ModeState) IsPersistent(key string) bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.persistentKeys[key]
}

// SetOnPersistDirty sets a callback invoked when a persistent key changes.
func (ms *ModeState) SetOnPersistDirty(fn func()) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.onPersistDirty = fn
}

// notifyPersistDirty calls the dirty callback if set. Caller must NOT hold the lock.
func (ms *ModeState) notifyPersistDirty() {
	ms.mu.RLock()
	fn := ms.onPersistDirty
	ms.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// PersistentKeys returns all keys currently marked as persistent.
func (ms *ModeState) PersistentKeys() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var keys []string
	for key := range ms.persistentKeys {
		keys = append(keys, key)
	}
	return keys
}

// ClearPersistentKey removes persistence and the value for a single key.
func (ms *ModeState) ClearPersistentKey(key string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.persistentKeys, key)
	delete(ms.values, key)
}

// ClearAllPersistent removes all persistent keys and their values.
func (ms *ModeState) ClearAllPersistent() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for key := range ms.persistentKeys {
		delete(ms.values, key)
	}
	ms.persistentKeys = make(map[string]bool)
}

// PersistentSnapshot returns all persistent key/value pairs as Go types for JSON serialization.
func (ms *ModeState) PersistentSnapshot() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	result := make(map[string]interface{})
	for key := range ms.persistentKeys {
		if val, ok := ms.values[key]; ok {
			result[key] = luaToGo(val)
		}
	}
	return result
}

// LoadPersistent loads persisted data into state as Lua values.
func (ms *ModeState) LoadPersistent(data map[string]interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for key, val := range data {
		ms.persistentKeys[key] = true
		ms.values[key] = ms.goToLuaDeep(val)
	}
}

// goToLuaDeep converts a Go value to Lua, including nested maps → tables.
// Must be called while holding ms.mu.
func (ms *ModeState) goToLuaDeep(v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case float64:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case map[string]interface{}:
		if ms.luaState == nil {
			return lua.LNil
		}
		tbl := ms.luaState.NewTable()
		for k, v := range val {
			tbl.RawSetString(k, ms.goToLuaDeep(v))
		}
		return tbl
	default:
		return lua.LNil
	}
}

// luaToGo converts a Lua value to a Go value for JSON serialization.
func luaToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LBool:
		return bool(val)
	case lua.LNumber:
		return float64(val)
	case *lua.LNilType:
		return nil
	case lua.LString:
		return string(val)
	case *lua.LTable:
		result := make(map[string]interface{})
		val.ForEach(func(key, value lua.LValue) {
			if ks, ok := key.(lua.LString); ok {
				result[string(ks)] = luaToGo(value)
			} else if kn, ok := key.(lua.LNumber); ok {
				result[fmt.Sprintf("%g", float64(kn))] = luaToGo(value)
			}
		})
		return result
	default:
		return nil
	}
}

// RegisterStateAPI registers the state global table in the Lua state.
// The state table has get(key) and set(key, value) functions, plus a
// metatable __index for read-only fields (mode, actions) and display declarations.
func RegisterStateAPI(L *lua.LState, ms *ModeState) {
	ms.mu.Lock()
	ms.luaState = L
	ms.mu.Unlock()

	// Store ModeState in Lua registry so it can be retrieved later
	ud := L.NewUserData()
	ud.Value = ms
	L.SetField(L.Get(lua.RegistryIndex), modeStateKey, ud)

	// Create the state table
	stateTbl := L.NewTable()

	// state.get(key) -> value
	stateTbl.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		ms.mu.RLock()
		val, ok := ms.values[key]
		ms.mu.RUnlock()
		if !ok {
			L.Push(lua.LNil)
		} else {
			L.Push(val)
		}
		return 1
	}))

	// state.set(key, value)
	stateTbl.RawSetString("set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := L.Get(2)
		ms.mu.Lock()
		ms.values[key] = val
		isPersistent := ms.persistentKeys[key]
		ms.mu.Unlock()
		if isPersistent {
			ms.notifyPersistDirty()
		}
		return 0
	}))

	// state.display(key, label) — declare a state item for sidebar display
	stateTbl.RawSetString("display", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		label := L.OptString(2, key)
		ms.AddDisplay(key, label)
		return 0
	}))

	// state.persist(key) — mark a key as persistent (survives clear and app restart)
	stateTbl.RawSetString("persist", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		ms.SetPersist(key)
		return 0
	}))

	// Metatable with __index for read-only fields
	mt := L.NewTable()
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)

		// Check for actions field
		if key == "actions" {
			ms.mu.RLock()
			tbl := ms.actions
			ms.mu.RUnlock()
			if tbl != nil {
				L.Push(tbl)
			} else {
				L.Push(lua.LNil)
			}
			return 1
		}

		// Check read-only fields
		ms.mu.RLock()
		val, ok := ms.readOnly[key]
		ms.mu.RUnlock()
		if ok {
			L.Push(val)
			return 1
		}

		L.Push(lua.LNil)
		return 1
	}))

	L.SetMetatable(stateTbl, mt)
	L.SetGlobal("state", stateTbl)
}

// GetModeState retrieves the ModeState from the Lua registry.
func GetModeState(L *lua.LState) *ModeState {
	ud := L.GetField(L.Get(lua.RegistryIndex), modeStateKey)
	if u, ok := ud.(*lua.LUserData); ok {
		if ms, ok := u.Value.(*ModeState); ok {
			return ms
		}
	}
	return nil
}

// goToLua converts a Go value to a Lua value.
func goToLua(v interface{}) lua.LValue {
	if v == nil {
		return lua.LNil
	}
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case int:
		return lua.LNumber(float64(val))
	case float64:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	default:
		return lua.LNil
	}
}
