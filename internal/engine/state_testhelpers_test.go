package engine

import lua "github.com/yuin/gopher-lua"

func (ms *ModeState) SetActions(actions []string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	if ms.luaState == nil {
		return
	}
	tbl := ms.luaState.NewTable()
	for i, action := range actions {
		tbl.RawSetInt(i+1, lua.LString(action))
	}
	ms.actions = tbl
}

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

func (ms *ModeState) GetValue(key string) (lua.LValue, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	value, ok := ms.values[key]
	return value, ok
}

func (ms *ModeState) IsPersistent(key string) bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.persistentKeys[key]
}

func (ms *ModeState) ClearAllPersistent() {
	ms.mu.Lock()
	for key := range ms.persistentKeys {
		delete(ms.values, key)
	}
	ms.persistentKeys = make(map[string]bool)
	ms.mu.Unlock()
	ms.notifyPersistDirty()
}

func GetModeState(L *lua.LState) *ModeState {
	ud := L.GetField(L.Get(lua.RegistryIndex), modeStateKey)
	if userData, ok := ud.(*lua.LUserData); ok {
		if state, ok := userData.Value.(*ModeState); ok {
			return state
		}
	}
	return nil
}
