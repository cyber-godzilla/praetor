package engine

import (
	"fmt"
	"log"

	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// Engine is the central automation engine that processes game text through
// Lua mode reactions.
// ModeChange describes a mode transition.
type ModeChange struct {
	NewMode  string
	PrevMode string
}

type Engine struct {
	mu           sync.Mutex
	vm           *LuaVM
	state        *ModeState
	queue        *CommandQueue
	metrics      *Metrics
	matcher      *Matcher
	status       *StatusValues
	timers       *TimerManager
	currentMode  string
	modeObj      *LuaMode
	modeChangeCh chan ModeChange // non-blocking notifications of mode switches
	dataDir      string
	persistStore *PersistentStore
	actionGen    uint64 // incremented on mode switch/reload to cancel stale delayed actions
}

// NewEngine creates a new Engine, initializes the Lua VM with bridge and state
// APIs, and loads all modes. Pass nil for cfg to use defaults.
func NewEngine(scriptDirs []string, cfg *config.Config, dataDir string) (*Engine, error) {
	var defaultDelay time.Duration
	var minInterval time.Duration
	var maxQueue int
	var highPriority []string

	if cfg != nil {
		defaultDelay = cfg.Commands.DefaultDelay.Duration
		minInterval = cfg.Commands.MinInterval.Duration
		maxQueue = cfg.Commands.MaxQueueSize
		highPriority = cfg.Commands.HighPriority
	} else {
		defaultDelay = 900 * time.Millisecond
		minInterval = 400 * time.Millisecond
		maxQueue = 20
		highPriority = []string{}
	}

	vm := NewLuaVM(scriptDirs)

	e := &Engine{
		vm:           vm,
		state:        NewModeState(),
		queue:        NewCommandQueue(maxQueue, defaultDelay, minInterval, highPriority),
		metrics:      NewMetrics(),
		matcher:      vm.Matcher(),
		status:       &StatusValues{},
		modeChangeCh: make(chan ModeChange, 16),
		dataDir:      dataDir,
	}

	L := vm.State()
	e.timers = NewTimerManager(L, &e.mu)
	RegisterBridge(L, e, e.status, e.timers)
	RegisterStateAPI(L, e.state)

	if err := vm.LoadModes(); err != nil {
		vm.Close()
		return nil, err
	}

	return e, nil
}

// SetUsername sets the authenticated username and loads persistent state from disk.
func (e *Engine) SetUsername(username string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.dataDir == "" || username == "" {
		return
	}

	e.persistStore = NewPersistentStore(e.dataDir, username)
	e.persistStore.SetSnapshotFunc(func() map[string]interface{} {
		return e.state.PersistentSnapshot()
	})

	e.state.SetOnPersistDirty(func() {
		if e.persistStore != nil {
			e.persistStore.MarkDirty()
		}
	})

	data, err := e.persistStore.Load()
	if err != nil {
		log.Printf("[ENGINE] loading persistent state: %v", err)
		return
	}
	if len(data) > 0 {
		e.state.LoadPersistent(data)
	}
}

// PersistentStore returns the persistent store, or nil if not yet initialized.
func (e *Engine) PersistentStore() *PersistentStore {
	return e.persistStore
}

// Close shuts down the engine and Lua VM.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.persistStore != nil {
		e.persistStore.Flush()
	}
	e.timers.ClearAll()
	if e.modeChangeCh != nil {
		close(e.modeChangeCh)
		e.modeChangeCh = nil
	}
	if e.vm != nil {
		e.vm.Close()
	}
}

// SetMode switches to a new mode, running on_stop on the previous mode and
// on_start on the new mode.
func (e *Engine) SetMode(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.setModeLocked(name, args)
}

// setModeLocked performs the mode switch while the lock is already held.
func (e *Engine) setModeLocked(name string, args []string) {
	L := e.vm.State()

	// Call on_stop on previous mode
	if e.modeObj != nil && e.modeObj.HasOnStop && e.modeObj.onStopRef != nil {
		if err := L.CallByParam(lua.P{
			Fn:      e.modeObj.onStopRef,
			NRet:    0,
			Protect: true,
		}); err != nil {
			log.Printf("[ENGINE] on_stop error for %s: %v", e.currentMode, err)
		}
	}

	// Cancel all active timers and delayed actions from previous mode.
	e.timers.ClearAll()
	e.actionGen++

	// End metrics session if one is active.
	e.metrics.EndSession()

	prevMode := e.currentMode

	// Clear state and queue
	e.state.Clear()
	e.queue.Clear()

	// Set new mode
	e.currentMode = name

	// Notify listeners about the mode change (non-blocking).
	select {
	case e.modeChangeCh <- ModeChange{NewMode: name, PrevMode: prevMode}:
	default:
	}

	if name == "" || name == "disable" {
		e.modeObj = nil
		e.state.SetReadOnly("mode", name)
		return
	}

	// Start a new metrics session for this mode.
	e.metrics.StartSession(name)

	mode, ok := e.vm.GetMode(name)
	if !ok {
		log.Printf("[ENGINE] mode %q not found", name)
		e.modeObj = nil
		e.state.SetReadOnly("mode", name)
		return
	}
	e.modeObj = mode

	// Update read-only state fields
	e.state.SetReadOnly("mode", name)

	// Call on_start with args
	if mode.onStartRef != nil {
		argsTbl := L.NewTable()
		for i, arg := range args {
			argsTbl.RawSetInt(i+1, lua.LString(arg))
		}

		if err := L.CallByParam(lua.P{
			Fn:      mode.onStartRef,
			NRet:    0,
			Protect: true,
		}, argsTbl); err != nil {
			log.Printf("[ENGINE] on_start error for %s: %v", name, err)
		}
	}
}

// Process is the main entry point: match game text against the current mode's
// reactions and execute the first matching action.
func (e *Engine) Process(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.modeObj == nil || len(e.modeObj.Reactions) == 0 {
		return
	}

	L := e.vm.State()

	for _, reaction := range e.modeObj.Reactions {
		// Match patterns (Go-side)
		if e.matcher.MatchAny(reaction.Patterns, text) < 0 {
			continue
		}

		// Check condition if present
		if reaction.HasCondition && reaction.conditionRef != nil {
			if err := L.CallByParam(lua.P{
				Fn:      reaction.conditionRef,
				NRet:    1,
				Protect: true,
			}); err != nil {
				log.Printf("[ENGINE] condition error: %v", err)
				continue
			}
			result := L.Get(-1)
			L.Pop(1)
			if result == lua.LFalse || result == lua.LNil {
				continue
			}
		}

		// Execute action, passing the matched text as argument.
		luaText := lua.LString(text)
		if reaction.DelayMs > 0 {
			// Copy references for the goroutine
			actionRef := reaction.actionRef
			delayMs := reaction.DelayMs
			capturedText := luaText
			gen := e.actionGen
			go func() {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
				e.mu.Lock()
				defer e.mu.Unlock()
				if e.actionGen != gen {
					return // mode switched or VM reloaded — discard stale action
				}
				if err := L.CallByParam(lua.P{
					Fn:      actionRef,
					NRet:    0,
					Protect: true,
				}, capturedText); err != nil {
					log.Printf("[ENGINE] delayed action error: %v", err)
				}
			}()
		} else {
			if err := L.CallByParam(lua.P{
				Fn:      reaction.actionRef,
				NRet:    0,
				Protect: true,
			}, luaText); err != nil {
				log.Printf("[ENGINE] action error: %v", err)
			}
		}

		// First matching reaction wins
		return
	}
}

// CurrentMode returns the name of the active mode.
func (e *Engine) CurrentMode() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentMode
}

// ModeChanges returns a channel that receives notifications of mode switches.
// This includes both API-initiated (SetMode) and Lua-initiated (set_mode) changes.
func (e *Engine) ModeChanges() <-chan ModeChange {
	return e.modeChangeCh
}

// Queue returns the command queue.
func (e *Engine) Queue() *CommandQueue {
	return e.queue
}

// Metrics returns the metrics tracker.
func (e *Engine) Metrics() *Metrics {
	return e.metrics
}

// State returns the mode state.
func (e *Engine) State() *ModeState {
	return e.state
}

// Status returns the status values (health, fatigue, etc.)
func (e *Engine) Status() *StatusValues {
	return e.status
}

// UpdateScriptDirs replaces the script directories and reloads all modes.
// If the current mode still exists after reload it is preserved; otherwise
// it is disabled.
func (e *Engine) UpdateScriptDirs(dirs []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.rebuildVM(dirs)
}

// ReloadMode hot-reloads a single mode from disk.
func (e *Engine) ReloadMode(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.vm.ReloadMode(name)
}

// ReloadAllModes rebuilds the Lua VM from scratch, rescanning all script
// directories for mode files. This discovers new files, removes deleted ones,
// and reloads changed ones.
func (e *Engine) ReloadAllModes() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.rebuildVM(e.vm.ScriptDirs())
}

// rebuildVM creates a fresh Lua VM with the given directories. If the current
// mode still exists after reload it is preserved; otherwise it is disabled.
// Must be called with e.mu held.
func (e *Engine) rebuildVM(dirs []string) error {
	prevMode := e.currentMode

	newVM := NewLuaVM(dirs)
	L := newVM.State()
	e.timers = NewTimerManager(L, &e.mu)
	RegisterBridge(L, e, e.status, e.timers)
	RegisterStateAPI(L, e.state)

	if err := newVM.LoadModes(); err != nil {
		newVM.Close()
		return fmt.Errorf("loading modes: %w", err)
	}

	// Cancel any delayed actions from the old VM before closing it.
	e.actionGen++

	if e.vm != nil {
		e.vm.Close()
	}
	e.vm = newVM
	e.matcher = newVM.Matcher()

	// Preserve current mode if it still exists, otherwise disable.
	if prevMode != "" && prevMode != "disable" {
		if _, ok := newVM.GetMode(prevMode); ok {
			e.currentMode = prevMode
			e.modeObj, _ = newVM.GetMode(prevMode)
		} else {
			e.setModeLocked("disable", nil)
		}
	} else {
		e.currentMode = prevMode
		e.modeObj = nil
	}

	return nil
}

// ModeNames returns the sorted names of all loaded modes.
func (e *Engine) ModeNames() []string {
	return e.vm.ModeNames()
}

// HasMode reports whether a mode with the given name is loaded.
func (e *Engine) HasMode(name string) bool {
	_, ok := e.vm.GetMode(name)
	return ok
}

// --- BridgeCallbacks implementation ---

// OnSend queues a command.
func (e *Engine) OnSend(command string, delayMs int) {
	e.queue.Enqueue(command, delayMs)
}

// OnSetMode switches to a new mode. Called from Lua while e.mu is held
// (during Process or on_start). Since setModeLocked also expects the lock
// held, we can call it directly without unlock/relock.
func (e *Engine) OnSetMode(mode string, args []string) {
	e.setModeLocked(mode, args)
}

// OnNotify logs a notification.
func (e *Engine) OnNotify(title, message string) {
	log.Printf("[NOTIFY] %s: %s", title, message)
}

// OnLog logs a message from Lua.
func (e *Engine) OnLog(message string) {
	log.Printf("[LUA] %s", message)
}

// OnMetricsTrack declares a metric for tracking.
func (e *Engine) OnMetricsTrack(key, label string) {
	e.metrics.Track(key, label)
}

// OnMetricsInc increments a metric.
func (e *Engine) OnMetricsInc(key string) {
	e.metrics.Inc(key)
}

// OnMetricsDec decrements a metric.
func (e *Engine) OnMetricsDec(key string) {
	e.metrics.Dec(key)
}

// OnMetricsSet sets a metric to a specific value.
func (e *Engine) OnMetricsSet(key string, value int) {
	e.metrics.Set(key, value)
}

// OnMetricsGet returns the current value of a metric.
func (e *Engine) OnMetricsGet(key string) int {
	return e.metrics.Get(key)
}

// SetHighPriority updates the command queue's high-priority list.
func (e *Engine) SetHighPriority(cmds []string) {
	e.queue.SetHighPriority(cmds)
}
