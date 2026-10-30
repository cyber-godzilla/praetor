package engine

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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
	valueOwners    map[string]*lua.LState
	readOnly       map[string]lua.LValue
	persistentKeys map[string]bool // keys that survive Clear()
	onPersistDirty func()          // called when a persistent key changes
	displayItems   []DisplayItem   // label→key mappings for sidebar/commands
	actions        *lua.LTable
	luaState       *lua.LState // reference for creating tables
	revision       atomic.Uint64
}

// NewModeState creates a new empty ModeState.
func NewModeState() *ModeState {
	return &ModeState{
		values:         make(map[string]lua.LValue),
		valueOwners:    make(map[string]*lua.LState),
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
	savedOwners := make(map[string]*lua.LState)
	for key := range ms.persistentKeys {
		if val, ok := ms.values[key]; ok {
			saved[key] = val
			savedOwners[key] = ms.valueOwners[key]
		}
	}

	ms.values = make(map[string]lua.LValue)
	ms.valueOwners = make(map[string]*lua.LState)
	for key, val := range saved {
		ms.values[key] = val
		ms.valueOwners[key] = savedOwners[key]
	}

	ms.displayItems = nil
	ms.actions = nil
	ms.revision.Add(1)
}

// AddDisplay registers a state key for sidebar display with a user-facing label.
func (ms *ModeState) AddDisplay(key, label string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	// Update existing if key already declared.
	for i, d := range ms.displayItems {
		if d.Key == key {
			ms.displayItems[i].Label = label
			ms.revision.Add(1)
			return
		}
	}
	ms.displayItems = append(ms.displayItems, DisplayItem{Key: key, Label: label})
	ms.revision.Add(1)
}

// Revision changes whenever data visible in the mode status snapshot changes.
func (ms *ModeState) Revision() uint64 { return ms.revision.Load() }

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
		ms.revision.Add(1)
		return
	}
	if val == lua.LTrue {
		ms.values[key] = lua.LFalse
	} else {
		ms.values[key] = lua.LTrue
	}
	ms.revision.Add(1)
}

// SetFromString sets a state value by key, parsing the string as number, bool, or string.
func (ms *ModeState) SetFromString(key, value string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.values[key] = parseStringToLua(value)
	ms.revision.Add(1)
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

// SetReadOnly sets a read-only field accessible as state.<key> in Lua.
// Accepts Go string, int, float64, bool, or nil.
func (ms *ModeState) SetReadOnly(key string, value interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.readOnly[key] = goToLua(value)
}

// SetPersist marks a key as persistent so it survives Clear().
func (ms *ModeState) SetPersist(key string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.persistentKeys[key] = true
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
	delete(ms.persistentKeys, key)
	delete(ms.values, key)
	delete(ms.valueOwners, key)
	ms.revision.Add(1)
	ms.mu.Unlock()
	// Mark dirty so the explicit Flush the UIs run persists the deletion;
	// otherwise the cleared key resurrects from disk on the next launch.
	ms.notifyPersistDirty()
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
		ms.valueOwners[key] = ms.luaState
	}
	ms.revision.Add(1)
}

// ReplacePersistent removes every value owned by the previous account before
// loading the next account's data. Non-persistent and read-only state are left
// alone; the normal disconnect/mode teardown owns those lifetimes.
func (ms *ModeState) ReplacePersistent(data map[string]interface{}) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for key := range ms.persistentKeys {
		delete(ms.values, key)
		delete(ms.valueOwners, key)
	}
	ms.persistentKeys = make(map[string]bool, len(data))
	for key, val := range data {
		ms.persistentKeys[key] = true
		ms.values[key] = ms.goToLuaDeep(val)
		ms.valueOwners[key] = ms.luaState
	}
	ms.revision.Add(1)
}

// RehomeValues copies portable state into a live replacement Lua state before
// an old VM is closed. Runtime reloads intentionally preserve mode state, but
// retaining tables owned by the retired VM would leave dangling cross-state
// objects. Capabilities and other non-data values are dropped.
func (ms *ModeState) RehomeValues(L *lua.LState) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for key, value := range ms.values {
		portable, ok := luaToPortable(value, make(map[*lua.LTable]bool))
		if !ok {
			delete(ms.values, key)
			delete(ms.valueOwners, key)
			continue
		}
		ms.values[key] = portableToLua(L, portable)
		ms.valueOwners[key] = L
	}
	ms.luaState = L
	ms.actions = nil
	ms.revision.Add(1)
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
		owner := ms.valueOwners[key]
		ms.mu.RUnlock()
		if !ok {
			L.Push(lua.LNil)
		} else {
			L.Push(luaValueForState(L, val, owner))
		}
		return 1
	}))

	// state.set(key, value)
	stateTbl.RawSetString("set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := L.Get(2)
		ms.mu.Lock()
		ms.values[key] = val
		ms.valueOwners[key] = L
		isPersistent := ms.persistentKeys[key]
		ms.revision.Add(1)
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

	// state.unpersist(key) removes a legacy durable value entirely. This is
	// primarily a migration tool for data that has moved to character variables;
	// ordinary transient state still disappears automatically on mode switch.
	stateTbl.RawSetString("unpersist", L.NewFunction(func(L *lua.LState) int {
		ms.ClearPersistentKey(L.CheckString(1))
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

// luaValueForState prevents values owned by one isolated Lua state from being
// pushed into another. Serializable data is copied without metatables;
// functions, userdata, threads, channels, and cycles become nil across the
// boundary, so Trusted capabilities cannot ride through state into Restricted.
func luaValueForState(L *lua.LState, value lua.LValue, owner *lua.LState) lua.LValue {
	if owner == nil || owner == L {
		return value
	}
	converted, ok := luaToPortable(value, make(map[*lua.LTable]bool))
	if !ok {
		return lua.LNil
	}
	return portableToLua(L, converted)
}

func luaToPortable(value lua.LValue, seen map[*lua.LTable]bool) (interface{}, bool) {
	switch typed := value.(type) {
	case *lua.LNilType:
		return nil, true
	case lua.LBool:
		return bool(typed), true
	case lua.LNumber:
		return float64(typed), true
	case lua.LString:
		return string(typed), true
	case *lua.LTable:
		if seen[typed] {
			return nil, false
		}
		seen[typed] = true
		defer delete(seen, typed)
		result := make(map[interface{}]interface{})
		valid := true
		typed.ForEach(func(key, item lua.LValue) {
			if !valid {
				return
			}
			var portableKey interface{}
			switch keyValue := key.(type) {
			case lua.LString:
				portableKey = string(keyValue)
			case lua.LNumber:
				portableKey = float64(keyValue)
			default:
				valid = false
				return
			}
			portableItem, ok := luaToPortable(item, seen)
			if !ok {
				valid = false
				return
			}
			result[portableKey] = portableItem
		})
		return result, valid
	default:
		return nil, false
	}
}

func portableToLua(L *lua.LState, value interface{}) lua.LValue {
	switch typed := value.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(typed)
	case float64:
		return lua.LNumber(typed)
	case string:
		return lua.LString(typed)
	case map[interface{}]interface{}:
		table := L.NewTable()
		for key, item := range typed {
			luaItem := portableToLua(L, item)
			switch keyValue := key.(type) {
			case string:
				table.RawSetString(keyValue, luaItem)
			case float64:
				table.RawSet(lua.LNumber(keyValue), luaItem)
			}
		}
		return table
	default:
		return lua.LNil
	}
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
