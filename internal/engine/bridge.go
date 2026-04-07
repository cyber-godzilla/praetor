package engine

import (
	"math/rand"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// BridgeCallbacks defines the interface for Go functions that Lua can invoke.
type BridgeCallbacks interface {
	OnSend(command string, delayMs int)
	OnSetMode(mode string, args []string)
	OnNotify(title, message string)
	OnLog(message string)
	OnMetricsTrack(key, label string)
	OnMetricsInc(key string)
	OnMetricsDec(key string)
	OnMetricsSet(key string, value int)
	OnMetricsGet(key string) int
}

// StatusValues holds the current status bar values, readable from Lua.
type StatusValues struct {
	mu          sync.RWMutex
	Health      int
	Fatigue     int
	Encumbrance int
	Satiation   int
}

// Update sets one or more status values atomically. Nil pointer fields are
// left unchanged.
func (sv *StatusValues) Update(health, fatigue, encumbrance, satiation *int) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	if health != nil {
		sv.Health = *health
	}
	if fatigue != nil {
		sv.Fatigue = *fatigue
	}
	if encumbrance != nil {
		sv.Encumbrance = *encumbrance
	}
	if satiation != nil {
		sv.Satiation = *satiation
	}
}

// Get returns the current health, fatigue, encumbrance, and satiation values.
func (sv *StatusValues) Get() (health, fatigue, encumbrance, satiation int) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.Health, sv.Fatigue, sv.Encumbrance, sv.Satiation
}

// RegisterBridge registers all Go functions as Lua globals.
// timers is optional; if non-nil, set_timeout, set_interval, and clear_timer
// are registered as Lua globals.
func RegisterBridge(L *lua.LState, cb BridgeCallbacks, status *StatusValues, timers ...*TimerManager) {
	// send(command) or send(command, delay_ms)
	L.SetGlobal("send", L.NewFunction(func(L *lua.LState) int {
		command := L.CheckString(1)
		delayMs := L.OptInt(2, 0)
		cb.OnSend(command, delayMs)
		return 0
	}))

	// set_mode(name) or set_mode(name, {args})
	L.SetGlobal("set_mode", L.NewFunction(func(L *lua.LState) int {
		mode := L.CheckString(1)
		var args []string
		if L.GetTop() >= 2 {
			tbl := L.OptTable(2, nil)
			if tbl != nil {
				tbl.ForEach(func(_, v lua.LValue) {
					args = append(args, lua.LVAsString(v))
				})
			}
		}
		cb.OnSetMode(mode, args)
		return 0
	}))

	// notify(title, message)
	L.SetGlobal("notify", L.NewFunction(func(L *lua.LState) int {
		title := L.CheckString(1)
		message := L.CheckString(2)
		cb.OnNotify(title, message)
		return 0
	}))

	// log(message)
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		message := L.CheckString(1)
		cb.OnLog(message)
		return 0
	}))

	// random_item(table) -> random element from array table
	L.SetGlobal("random_item", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		length := tbl.Len()
		if length == 0 {
			L.Push(lua.LNil)
			return 1
		}
		idx := rand.Intn(length) + 1
		L.Push(tbl.RawGetInt(idx))
		return 1
	}))

	// time table: time.now(), time.since(ts)
	timeTbl := L.NewTable()
	timeTbl.RawSetString("now", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(float64(time.Now().UnixMilli())))
		return 1
	}))
	timeTbl.RawSetString("since", L.NewFunction(func(L *lua.LState) int {
		ts := L.CheckNumber(1)
		start := time.UnixMilli(int64(ts))
		elapsed := time.Since(start).Milliseconds()
		L.Push(lua.LNumber(float64(elapsed)))
		return 1
	}))
	L.SetGlobal("time", timeTbl)

	// status table with read-only fields via metatable
	statusTbl := L.NewTable()
	statusMT := L.NewTable()
	statusMT.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		status.mu.RLock()
		defer status.mu.RUnlock()
		switch key {
		case "health":
			L.Push(lua.LNumber(float64(status.Health)))
		case "fatigue":
			L.Push(lua.LNumber(float64(status.Fatigue)))
		case "encumbrance":
			L.Push(lua.LNumber(float64(status.Encumbrance)))
		case "satiation":
			L.Push(lua.LNumber(float64(status.Satiation)))
		default:
			L.Push(lua.LNil)
		}
		return 1
	}))
	L.SetMetatable(statusTbl, statusMT)
	L.SetGlobal("status", statusTbl)

	// metrics table
	metricsTbl := L.NewTable()
	metricsTbl.RawSetString("track", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		label := L.OptString(2, key)
		cb.OnMetricsTrack(key, label)
		return 0
	}))
	metricsTbl.RawSetString("inc", L.NewFunction(func(L *lua.LState) int {
		cb.OnMetricsInc(L.CheckString(1))
		return 0
	}))
	metricsTbl.RawSetString("dec", L.NewFunction(func(L *lua.LState) int {
		cb.OnMetricsDec(L.CheckString(1))
		return 0
	}))
	metricsTbl.RawSetString("set", L.NewFunction(func(L *lua.LState) int {
		cb.OnMetricsSet(L.CheckString(1), L.CheckInt(2))
		return 0
	}))
	metricsTbl.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(cb.OnMetricsGet(L.CheckString(1))))
		return 1
	}))
	L.SetGlobal("metrics", metricsTbl)

	// Timer functions (optional)
	if len(timers) > 0 && timers[0] != nil {
		tm := timers[0]

		// set_timeout(callback, delay_ms) -> timer_id
		L.SetGlobal("set_timeout", L.NewFunction(func(L *lua.LState) int {
			callback := L.CheckFunction(1)
			delayMs := L.CheckInt(2)
			id := tm.SetTimeout(callback, delayMs)
			L.Push(lua.LNumber(id))
			return 1
		}))

		// set_interval(callback, interval_ms) -> timer_id
		L.SetGlobal("set_interval", L.NewFunction(func(L *lua.LState) int {
			callback := L.CheckFunction(1)
			intervalMs := L.CheckInt(2)
			id := tm.SetInterval(callback, intervalMs)
			L.Push(lua.LNumber(id))
			return 1
		}))

		// clear_timer(id)
		L.SetGlobal("clear_timer", L.NewFunction(func(L *lua.LState) int {
			id := L.CheckInt(1)
			tm.ClearTimer(id)
			return 0
		}))
	}
}
