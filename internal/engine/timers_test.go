package engine

import (
	"sync"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func newTestTimerManager(t *testing.T) (*TimerManager, *lua.LState, *sync.Mutex) {
	t.Helper()
	L := lua.NewState()
	mu := &sync.Mutex{}
	tm := NewTimerManager(L, mu)
	t.Cleanup(func() {
		tm.ClearAll()
		L.Close()
	})
	return tm, L, mu
}

func TestSetTimeout(t *testing.T) {
	tm, L, mu := newTestTimerManager(t)

	// Register a Lua callback that sets a global variable
	mu.Lock()
	err := L.DoString(`fired = false`)
	mu.Unlock()
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	mu.Lock()
	fn := L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("fired", lua.LTrue)
		return 0
	})
	mu.Unlock()

	tm.SetTimeout(fn, 50)

	// Wait for timer to fire
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	val := L.GetGlobal("fired")
	mu.Unlock()

	if val != lua.LTrue {
		t.Errorf("expected fired=true after timeout, got %v", val)
	}
}

func TestSetInterval(t *testing.T) {
	tm, L, mu := newTestTimerManager(t)

	// Register a Lua counter
	mu.Lock()
	err := L.DoString(`counter = 0`)
	mu.Unlock()
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	mu.Lock()
	fn := L.NewFunction(func(L *lua.LState) int {
		val := L.GetGlobal("counter")
		n := lua.LVAsNumber(val)
		L.SetGlobal("counter", lua.LNumber(n+1))
		return 0
	})
	mu.Unlock()

	id := tm.SetInterval(fn, 50)

	// Wait for at least 3 ticks
	time.Sleep(175 * time.Millisecond)

	mu.Lock()
	val := L.GetGlobal("counter")
	mu.Unlock()
	count := int(lua.LVAsNumber(val))

	if count < 3 {
		t.Errorf("expected counter >= 3 after 175ms at 50ms interval, got %d", count)
	}

	// Clear the timer and check counter doesn't increase much
	tm.ClearTimer(id)
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	val2 := L.GetGlobal("counter")
	mu.Unlock()
	countAfter := int(lua.LVAsNumber(val2))

	// Allow at most 1 extra tick due to timing
	if countAfter > count+1 {
		t.Errorf("counter increased from %d to %d after clearing interval", count, countAfter)
	}
}

func TestClearAll(t *testing.T) {
	tm, L, mu := newTestTimerManager(t)

	mu.Lock()
	err := L.DoString(`fired1 = false; fired2 = false; fired3 = false`)
	mu.Unlock()
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	mu.Lock()
	fn1 := L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("fired1", lua.LTrue)
		return 0
	})
	fn2 := L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("fired2", lua.LTrue)
		return 0
	})
	fn3 := L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("fired3", lua.LTrue)
		return 0
	})
	mu.Unlock()

	tm.SetTimeout(fn1, 100)
	tm.SetInterval(fn2, 100)
	tm.SetTimeout(fn3, 100)

	// Clear all immediately
	tm.ClearAll()

	// Wait for the timers to have fired if they weren't cancelled
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	v1 := L.GetGlobal("fired1")
	v2 := L.GetGlobal("fired2")
	v3 := L.GetGlobal("fired3")
	mu.Unlock()

	if v1 == lua.LTrue {
		t.Error("fired1 should be false after ClearAll")
	}
	if v2 == lua.LTrue {
		t.Error("fired2 should be false after ClearAll")
	}
	if v3 == lua.LTrue {
		t.Error("fired3 should be false after ClearAll")
	}
}

func TestTimerDoesNotFireAfterClear(t *testing.T) {
	tm, L, mu := newTestTimerManager(t)

	mu.Lock()
	err := L.DoString(`fired = false`)
	mu.Unlock()
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	mu.Lock()
	fn := L.NewFunction(func(L *lua.LState) int {
		L.SetGlobal("fired", lua.LTrue)
		return 0
	})
	mu.Unlock()

	id := tm.SetTimeout(fn, 100)

	// Clear at 50ms
	time.Sleep(50 * time.Millisecond)
	tm.ClearTimer(id)

	// Wait well past the original fire time
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	val := L.GetGlobal("fired")
	mu.Unlock()

	if val == lua.LTrue {
		t.Error("timer fired after being cleared")
	}
}
