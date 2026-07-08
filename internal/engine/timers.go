package engine

import (
	"log"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// timerEntry represents a single active timer (one-shot or repeating).
type timerEntry struct {
	id       int
	interval time.Duration // 0 for one-shot
	callback *lua.LFunction
	cancel   chan struct{}
}

// TimerManager provides goroutine-based timers that call Lua callbacks.
// Each timer runs in its own goroutine. All Lua VM access is serialized
// through luaMu (the engine's mutex).
type TimerManager struct {
	mu       sync.Mutex
	timers   map[int]*timerEntry
	nextID   int
	luaState *lua.LState
	luaMu    *sync.Mutex
	// closed marks the manager's Lua state as torn down. It is written under
	// luaMu (via Shutdown) and read under luaMu by each timer goroutine before
	// it calls into the state, so a fired timer can never dereference a closed
	// LState (which panics with a nil pointer inside gopher-lua).
	closed bool
}

// NewTimerManager creates a TimerManager bound to the given Lua state.
// luaMu must be the engine's mutex that protects all Lua VM access.
func NewTimerManager(L *lua.LState, luaMu *sync.Mutex) *TimerManager {
	return &TimerManager{
		timers:   make(map[int]*timerEntry),
		nextID:   1,
		luaState: L,
		luaMu:    luaMu,
	}
}

// SetTimeout schedules a one-shot callback after delayMs milliseconds.
// Returns a timer ID that can be passed to ClearTimer.
func (tm *TimerManager) SetTimeout(callback *lua.LFunction, delayMs int) int {
	tm.mu.Lock()
	id := tm.nextID
	tm.nextID++
	entry := &timerEntry{
		id:       id,
		interval: 0,
		callback: callback,
		cancel:   make(chan struct{}),
	}
	tm.timers[id] = entry
	tm.mu.Unlock()

	go func() {
		select {
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
			// Check cancel before executing
			tm.mu.Lock()
			_, alive := tm.timers[id]
			if !alive {
				tm.mu.Unlock()
				return
			}
			// Remove from map before executing (one-shot)
			delete(tm.timers, id)
			tm.mu.Unlock()

			// Acquire Lua mutex and execute callback. Bail if the state was
			// torn down (Shutdown) while we were waiting for the lock.
			tm.luaMu.Lock()
			if tm.closed {
				tm.luaMu.Unlock()
				return
			}
			if err := tm.luaState.CallByParam(lua.P{
				Fn:      callback,
				NRet:    0,
				Protect: true,
			}); err != nil {
				log.Printf("[TIMER] timeout callback error: %v", err)
			}
			tm.luaMu.Unlock()
		case <-entry.cancel:
			return
		}
	}()

	return id
}

// SetInterval schedules a repeating callback every intervalMs milliseconds.
// Returns a timer ID that can be passed to ClearTimer.
func (tm *TimerManager) SetInterval(callback *lua.LFunction, intervalMs int) int {
	tm.mu.Lock()
	id := tm.nextID
	tm.nextID++
	entry := &timerEntry{
		id:       id,
		interval: time.Duration(intervalMs) * time.Millisecond,
		callback: callback,
		cancel:   make(chan struct{}),
	}
	tm.timers[id] = entry
	tm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(entry.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check cancel before executing
				tm.mu.Lock()
				_, alive := tm.timers[id]
				tm.mu.Unlock()
				if !alive {
					return
				}

				// Acquire Lua mutex and execute callback. Bail if the state was
				// torn down (Shutdown) while we were waiting for the lock.
				tm.luaMu.Lock()
				if tm.closed {
					tm.luaMu.Unlock()
					return
				}
				if err := tm.luaState.CallByParam(lua.P{
					Fn:      callback,
					NRet:    0,
					Protect: true,
				}); err != nil {
					log.Printf("[TIMER] interval callback error: %v", err)
				}
				tm.luaMu.Unlock()
			case <-entry.cancel:
				return
			}
		}
	}()

	return id
}

// ClearTimer cancels and removes the timer with the given ID.
func (tm *TimerManager) ClearTimer(id int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	entry, ok := tm.timers[id]
	if !ok {
		return
	}
	close(entry.cancel)
	delete(tm.timers, id)
}

// ClearAll cancels all active timers. Used on mode switch, where the Lua state
// itself stays alive (only the current mode's timers are dropped).
func (tm *TimerManager) ClearAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, entry := range tm.timers {
		close(entry.cancel)
		delete(tm.timers, id)
	}
}

// Shutdown permanently retires the manager because its Lua state is being
// closed or replaced. It marks the manager closed and cancels every timer so
// no goroutine calls into the (soon-to-be) dead state. MUST be called with
// luaMu held (i.e. the engine mutex), so it is ordered against the closed-check
// each timer goroutine performs under luaMu before invoking its callback.
func (tm *TimerManager) Shutdown() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.closed = true
	for id, entry := range tm.timers {
		close(entry.cancel)
		delete(tm.timers, id)
	}
}
