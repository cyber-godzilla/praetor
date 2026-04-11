package engine

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// LuaReaction is a parsed reaction from a Lua mode table.
type LuaReaction struct {
	RawPatterns  []string
	Patterns     []CompiledPattern
	HasCondition bool
	DelayMs      int
	actionRef    *lua.LFunction
	conditionRef *lua.LFunction
}

// LuaMode is a loaded mode.
type LuaMode struct {
	Name       string
	Reactions  []LuaReaction
	HasOnStop  bool
	onStartRef *lua.LFunction
	onStopRef  *lua.LFunction
}

// LuaVM manages the gopher-lua state, mode registry, and hot reload.
type LuaVM struct {
	mu         sync.Mutex
	state      *lua.LState
	modes      map[string]*LuaMode
	scriptDirs []string
	matcher    *Matcher
}

// NewLuaVM creates a new LuaVM with a fresh Lua state. All script directories
// are added to package.path so that modes can require shared libraries.
func NewLuaVM(scriptDirs []string) *LuaVM {
	L := lua.NewState()

	// Add all script directories to package.path.
	pkg := L.GetGlobal("package")
	if tbl, ok := pkg.(*lua.LTable); ok {
		currentPath := lua.LVAsString(tbl.RawGetString("path"))
		var newPaths []string
		for _, dir := range scriptDirs {
			newPaths = append(newPaths, filepath.Join(dir, "?.lua"))
			newPaths = append(newPaths, filepath.Join(dir, "?/init.lua"))
		}
		if currentPath != "" {
			newPaths = append(newPaths, currentPath)
		}
		tbl.RawSetString("path", lua.LString(strings.Join(newPaths, ";")))
	}

	return &LuaVM{
		state:      L,
		modes:      make(map[string]*LuaMode),
		scriptDirs: scriptDirs,
		matcher:    NewMatcher(),
	}
}

// Close shuts down the Lua state.
func (vm *LuaVM) Close() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if vm.state != nil {
		vm.state.Close()
		vm.state = nil
	}
}

// LoadModes scans all script directories for .lua files, loads each one, and
// registers the returned mode table. Files are loaded in sorted order per
// directory for determinism. Later directories can override earlier ones.
func (vm *LuaVM) LoadModes() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, dir := range vm.scriptDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			log.Printf("[ENGINE] skipping script directory %s: %v", dir, err)
			continue
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".lua") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".lua")
			path := filepath.Join(dir, entry.Name())

			mode, err := vm.loadModeFile(name, path)
			if err != nil {
				log.Printf("[ENGINE] skipping %s: %v", path, err)
				continue
			}
			vm.modes[name] = mode
		}
	}

	return nil
}

// GetMode returns the named mode and whether it exists.
func (vm *LuaVM) GetMode(name string) (*LuaMode, bool) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	m, ok := vm.modes[name]
	return m, ok
}

// ReloadMode re-reads a single mode file from disk and replaces the loaded
// mode. Shared libraries are uncached first so require() picks up changes.
// The matcher cache is cleared since patterns may have changed.
func (vm *LuaVM) ReloadMode(name string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.clearLibCache()

	for _, dir := range vm.scriptDirs {
		path := filepath.Join(dir, name+".lua")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		mode, err := vm.loadModeFile(name, path)
		if err != nil {
			return fmt.Errorf("reloading mode %s: %w", name, err)
		}
		vm.modes[name] = mode
		vm.matcher.ClearCache()
		return nil
	}

	return fmt.Errorf("mode %s not found in any script directory", name)
}

// ScriptDirs returns the configured script directories.
func (vm *LuaVM) ScriptDirs() []string {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.scriptDirs
}

// ModeFileExists checks whether the mode's .lua file exists in any script directory.
func (vm *LuaVM) ModeFileExists(name string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, dir := range vm.scriptDirs {
		path := filepath.Join(dir, name+".lua")
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// RemoveMode removes a mode from the loaded modes map.
func (vm *LuaVM) RemoveMode(name string) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	delete(vm.modes, name)
}

// clearLibCache removes all entries from Lua's package.loaded table
// so that subsequent require() calls re-read files from disk.
func (vm *LuaVM) clearLibCache() {
	pkg := vm.state.GetGlobal("package")
	tbl, ok := pkg.(*lua.LTable)
	if !ok {
		return
	}
	loaded, ok := tbl.RawGetString("loaded").(*lua.LTable)
	if !ok {
		return
	}
	var keys []string
	loaded.ForEach(func(key, _ lua.LValue) {
		if s, ok := key.(lua.LString); ok {
			keys = append(keys, string(s))
		}
	})
	for _, k := range keys {
		loaded.RawSetString(k, lua.LNil)
	}
}

// ModeNames returns a sorted list of loaded mode names.
func (vm *LuaVM) ModeNames() []string {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	names := make([]string, 0, len(vm.modes))
	for name := range vm.modes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// State returns the underlying lua.LState for bridge registration.
func (vm *LuaVM) State() *lua.LState {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.state
}

// Matcher returns the VM's pattern matcher.
func (vm *LuaVM) Matcher() *Matcher {
	return vm.matcher
}

// loadModeFile loads a single .lua file which must return a table M with
// on_start(args), optional on_stop(), and reactions array.
func (vm *LuaVM) loadModeFile(name, path string) (*LuaMode, error) {
	if err := vm.state.DoFile(path); err != nil {
		return nil, fmt.Errorf("executing %s: %w", path, err)
	}

	// The file should push its return value onto the stack
	ret := vm.state.Get(-1)
	vm.state.Pop(1)

	tbl, ok := ret.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s: expected table return, got %s", path, ret.Type())
	}

	mode := &LuaMode{
		Name: name,
	}

	// Extract on_start
	if fn, ok := tbl.RawGetString("on_start").(*lua.LFunction); ok {
		mode.onStartRef = fn
	}

	// Extract on_stop
	if fn, ok := tbl.RawGetString("on_stop").(*lua.LFunction); ok {
		mode.onStopRef = fn
		mode.HasOnStop = true
	}

	// Extract reactions
	reactionsVal := tbl.RawGetString("reactions")
	if reactionsTbl, ok := reactionsVal.(*lua.LTable); ok {
		var reactions []LuaReaction
		reactionsTbl.ForEach(func(_, value lua.LValue) {
			if rTbl, ok := value.(*lua.LTable); ok {
				reaction, err := vm.parseReaction(rTbl)
				if err == nil {
					reactions = append(reactions, reaction)
				}
			}
		})
		mode.Reactions = reactions
	}

	return mode, nil
}

// parseReaction extracts a LuaReaction from a Lua table with match, action,
// optional condition, and optional delay fields.
func (vm *LuaVM) parseReaction(tbl *lua.LTable) (LuaReaction, error) {
	reaction := LuaReaction{}

	// Parse match: can be a string or table of strings
	matchVal := tbl.RawGetString("match")
	switch v := matchVal.(type) {
	case lua.LString:
		pattern := string(v)
		reaction.RawPatterns = []string{pattern}
		reaction.Patterns = []CompiledPattern{vm.matcher.Compile(pattern)}
	case *lua.LTable:
		v.ForEach(func(_, val lua.LValue) {
			if s, ok := val.(lua.LString); ok {
				pattern := string(s)
				reaction.RawPatterns = append(reaction.RawPatterns, pattern)
				reaction.Patterns = append(reaction.Patterns, vm.matcher.Compile(pattern))
			}
		})
	default:
		return reaction, fmt.Errorf("match field must be string or table, got %s", matchVal.Type())
	}

	// Parse action (required)
	if fn, ok := tbl.RawGetString("action").(*lua.LFunction); ok {
		reaction.actionRef = fn
	} else {
		return reaction, fmt.Errorf("action field must be a function")
	}

	// Parse condition (optional)
	if fn, ok := tbl.RawGetString("condition").(*lua.LFunction); ok {
		reaction.conditionRef = fn
		reaction.HasCondition = true
	}

	// Parse delay (optional)
	if delayVal, ok := tbl.RawGetString("delay").(lua.LNumber); ok {
		reaction.DelayMs = int(delayVal)
	}

	return reaction, nil
}
