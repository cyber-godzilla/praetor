package engine

import (
	"fmt"
	"log"
	"strings"
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
	notifyMu     sync.RWMutex
	notifySink   func(title, message string)
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

	inSwitch bool           // true while setModeLocked's on_start/on_stop are running
	pending  *pendingSwitch // a set_mode requested during an active switch (deferred)
}

// SetNotificationSink installs the shell-specific delivery path for Lua's
// notify(title, message). It is independent of the engine lock because Lua
// callbacks execute while that lock is already held.
func (e *Engine) SetNotificationSink(sink func(title, message string)) {
	e.notifyMu.Lock()
	e.notifySink = sink
	e.notifyMu.Unlock()
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
	e.timers = NewTimerManager(L, &e.mu, &e.actionGen)
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
		// Acquire the engine mutex before snapshotting: the debounced flush runs
		// on a timer goroutine, and the snapshot iterates live Lua tables
		// (LTable.ForEach) that reactions mutate under e.mu. Without this lock the
		// two race → "fatal error: concurrent map iteration and map write".
		// Callers must therefore NOT hold e.mu when triggering a flush (see Close).
		e.mu.Lock()
		defer e.mu.Unlock()
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
	// Flush persistent state before taking e.mu: the snapshot function acquires
	// e.mu itself, so flushing under the lock here would deadlock. Grab the store
	// pointer under a brief lock, then flush without it.
	e.mu.Lock()
	ps := e.persistStore
	e.mu.Unlock()
	if ps != nil {
		ps.Flush()
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	// Shutdown (not ClearAll): the VM is closing, so mark the manager dead so
	// no in-flight timer goroutine calls into the closed Lua state. Also advance
	// the generation so any timer/action mid-flight is retired even if it somehow
	// slips past the closed check.
	e.timers.Shutdown()
	e.actionGen++
	if e.modeChangeCh != nil {
		close(e.modeChangeCh)
		e.modeChangeCh = nil
	}
	if e.vm != nil {
		e.vm.Close()
	}
}

// maxModeSwitchHops caps deferred set_mode chains so a self-referential or
// ping-ponging on_start/on_stop can't churn (previously ~128 nested switches,
// each ending/starting a metric session, before gopher-lua's stack overflow).
const maxModeSwitchHops = 8

type pendingSwitch struct {
	name string
	args []string
}

// SetMode switches to a new mode, running on_stop on the previous mode and
// on_start on the new mode.
func (e *Engine) SetMode(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.runSwitch(name, args)
}

// runSwitch performs a mode switch and then applies any switch that on_start/
// on_stop requested via set_mode (deferred, last-writer-wins), iteratively
// rather than recursively. The chain is capped; exceeding it logs a loop error
// and stops, leaving the engine on the last completed mode and usable.
func (e *Engine) runSwitch(name string, args []string) {
	hops := 0
	for {
		e.inSwitch = true
		e.setModeLocked(name, args)
		e.inSwitch = false

		if e.pending == nil {
			return
		}
		next := e.pending
		e.pending = nil
		hops++
		if hops > maxModeSwitchHops {
			log.Printf("[ENGINE] mode switch loop detected (>%d hops); stopping at %q, ignoring pending %q",
				maxModeSwitchHops, e.currentMode, next.name)
			return
		}
		name, args = next.name, next.args
	}
}

// setModeLocked performs the mode switch while the lock is already held.
func (e *Engine) setModeLocked(name string, args []string) {
	L := e.vm.State()

	// Normalize the requested mode to its canonical (case-correct) name so
	// currentMode, the metrics session, and the mode-change event all agree
	// regardless of how the caller cased it. "" and "disable" have no canonical
	// form and pass through unchanged.
	if name != "" && name != "disable" {
		if canonical, ok := e.vm.ResolveModeName(name); ok {
			name = canonical
		}
	}

	// Discard the outgoing mode's still-pending commands *before* running its
	// on_stop, so on_stop's own cleanup sends (sheathe/stand patterns) survive
	// into the new mode's queue instead of being enqueued and then wiped. The
	// generation bump inside Clear() also lets the drainer recall any old-mode
	// command it dequeued but is still sleeping on.
	e.queue.Clear()

	// Call on_stop on previous mode
	if e.modeObj != nil && e.modeObj.HasOnStop && e.modeObj.onStopRef != nil {
		if err := callLua(L, lua.P{
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

	// Clear state (on_stop above has already run against the old state).
	e.state.Clear()

	// Set new mode
	e.currentMode = name

	// Notify listeners about the mode change. This channel is coalescing: a
	// consumer only needs the latest mode, so when the buffer is full we drop the
	// OLDEST pending change to make room for this one. That guarantees the final
	// mode of a rapid set_mode burst always lands (dropping intermediate ones is
	// fine) instead of the newest being the one discarded.
	mc := ModeChange{NewMode: name, PrevMode: prevMode}
	select {
	case e.modeChangeCh <- mc:
	default:
		select {
		case <-e.modeChangeCh:
		default:
		}
		select {
		case e.modeChangeCh <- mc:
		default:
		}
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

		if err := callLua(L, lua.P{
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
			if err := callLua(L, lua.P{
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
				if err := callLua(L, lua.P{
					Fn:      actionRef,
					NRet:    0,
					Protect: true,
				}, capturedText); err != nil {
					log.Printf("[ENGINE] delayed action error: %v", err)
				}
			}()
		} else {
			if err := callLua(L, lua.P{
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
	newTimers := NewTimerManager(L, &e.mu, &e.actionGen)
	RegisterBridge(L, e, e.status, newTimers)
	RegisterStateAPI(L, e.state)

	if err := newVM.LoadModes(); err != nil {
		newTimers.Shutdown()
		newVM.Close()
		return fmt.Errorf("loading modes: %w", err)
	}

	// Only retire the working VM/timers after the replacement has loaded
	// successfully. A syntax error in a newly configured directory therefore
	// leaves the current automation session intact.
	e.timers.Shutdown()
	e.timers = newTimers

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

// ModeNames returns the sorted names of all loaded modes, excluding library
// modules (names prefixed "lib_"). Libraries are require()d by real modes and
// are not player-selectable, so they are hidden from every mode listing (GUI
// modals, sidebar, quick-cycle pickers, and the TUI mode picker). Mode-setting
// validation goes through HasMode, which is unaffected.
func (e *Engine) ModeNames() []string {
	all := e.vm.ModeNames()
	names := make([]string, 0, len(all))
	for _, n := range all {
		if strings.HasPrefix(n, "lib_") {
			continue
		}
		names = append(names, n)
	}
	return names
}

// HasMode reports whether a mode matching the given name (case-insensitively) is
// loaded.
func (e *Engine) HasMode(name string) bool {
	_, ok := e.vm.ResolveModeName(name)
	return ok
}

// --- BridgeCallbacks implementation ---

// OnSend queues a command.
func (e *Engine) OnSend(command string, delayMs int) {
	e.queue.Enqueue(command, delayMs)
}

// OnSetMode switches to a new mode. Called from Lua while e.mu is held (during
// Process, on_start, or on_stop). A set_mode issued *during* an active switch is
// deferred (recorded, last-writer-wins) and applied by runSwitch once the current
// switch finishes, so a recursive/ping-ponging on_start can't blow the stack.
func (e *Engine) OnSetMode(mode string, args []string) {
	if e.inSwitch {
		e.pending = &pendingSwitch{name: mode, args: args}
		return
	}
	e.runSwitch(mode, args)
}

// OnNotify logs and delivers a notification through the active shell sink.
func (e *Engine) OnNotify(title, message string) {
	log.Printf("[NOTIFY] %s: %s", title, message)
	e.notifyMu.RLock()
	sink := e.notifySink
	e.notifyMu.RUnlock()
	if sink != nil {
		sink(title, message)
	}
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
