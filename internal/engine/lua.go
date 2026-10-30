package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// scriptTimeout bounds a single entry into Lua. A runaway script
// (`while true do end`) in any reaction, condition, timer, or lifecycle hook
// would otherwise hang the engine forever — and because Process holds e.mu
// across all Lua execution, stall the whole line pipeline until the process is
// killed. It is a package var (not const) so tests can shrink it.
var scriptTimeout = 2 * time.Second

// callLua invokes a Lua callback with an execution deadline. On expiry the VM
// aborts the call and returns a context error, which callers log and treat like
// any other Lua runtime error (they already use Protect'd pcall paths).
//
// Lua entries NEST: a set_mode from a reaction/condition/timer runs on_stop/
// on_start via callLua on the same LState. So we save the caller's context and
// RESTORE it on return rather than unconditionally removing it — an
// unconditional RemoveContext would nil L.ctx while the outer mainLoopWithContext
// is still running, and its per-instruction `<-L.ctx.Done()` would then
// dereference a nil context and panic (dropping everything after the nested
// call). At the outermost entry prev is nil, so we RemoveContext as before.
func callLua(L *lua.LState, p lua.P, args ...lua.LValue) error {
	prev := L.Context()
	ctx, cancel := context.WithTimeout(context.Background(), scriptTimeout)
	defer cancel()
	L.SetContext(ctx)
	defer func() {
		if prev != nil {
			L.SetContext(prev)
		} else {
			L.RemoveContext()
		}
	}()
	return L.CallByParam(p, args...)
}

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
	Usage      string
	Desc       string
	Chains     bool
	Hidden     bool
	Reactions  []LuaReaction
	HasOnStop  bool
	onStartRef *lua.LFunction
	onStopRef  *lua.LFunction
	state      *lua.LState
	SourcePath string
	Trust      string
}

// ModeSpec is a mode's self-declared metadata, read off the mode table at load
// time. It exists so a shell can describe a mode — its arguments and what it
// does — without running it: everything a mode registers at runtime
// (metrics.track, state.display) is only known once on_start has fired.
//
// All fields are optional. An empty Usage means the mode takes no arguments,
// and Desc is the signal that a mode declared anything at all.
type ModeSpec struct {
	Name string `json:"name"`
	// Usage is the argument signature without the mode name, e.g.
	// "<item> [corpse#]".
	Usage string `json:"usage"`
	// Desc is a one-line description of what the mode does.
	Desc string `json:"desc"`
	// Chains reports that the mode honors an after_mode/after_do/after_ps completion
	// handoff. It is
	// reported, never acted on — appending the token to a displayed signature
	// is the shell's business, and the chaining itself lives in Lua.
	Chains bool `json:"chains"`
	// Hidden asks the command hint not to offer this mode, for helpers that are
	// real modes but noise while typing — an internal route leg, a mode another
	// mode chains into. It is a display hint only: the mode stays loaded and
	// "/mode <name>" still runs it, because mode resolution goes through
	// HasMode, which never consults this.
	Hidden bool `json:"hidden"`
}

// LuaVM manages the gopher-lua state, mode registry, and hot reload.
type LuaVM struct {
	mu       sync.Mutex
	state    *lua.LState // compatibility: trusted state, or first restricted state
	states   []*lua.LState
	modes    map[string]*LuaMode
	sources  []LuaSource
	runtimes map[string]*lua.LState
	matcher  *Matcher
}

const (
	LuaTrustRestricted = "restricted"
	LuaTrustTrusted    = "trusted"
)

// LuaSource assigns one script root to a source-level trust boundary. Trusted
// roots share the legacy Lua state for cross-directory compatibility;
// Restricted roots receive independent states and source-scoped module caches.
type LuaSource struct {
	Path  string
	Trust string
}

// NewLuaVM creates a new LuaVM with a fresh Lua state. All script directories
// are added to package.path so that modes can require shared libraries.
func NewLuaVM(scriptDirs []string) *LuaVM {
	sources := make([]LuaSource, 0, len(scriptDirs))
	for _, dir := range scriptDirs {
		sources = append(sources, LuaSource{Path: dir, Trust: LuaTrustTrusted})
	}
	return NewLuaVMWithSources(sources)
}

func NewLuaVMWithSources(sources []LuaSource) *LuaVM {
	vm := &LuaVM{
		modes: make(map[string]*LuaMode), runtimes: make(map[string]*lua.LState), matcher: NewMatcher(),
	}
	trustedDirs := make([]string, 0, len(sources))
	for _, source := range sources {
		source.Path = filepath.Clean(source.Path)
		if source.Trust != LuaTrustRestricted {
			source.Trust = LuaTrustTrusted
			trustedDirs = append(trustedDirs, source.Path)
		}
		vm.sources = append(vm.sources, source)
	}
	var trusted *lua.LState
	if len(trustedDirs) > 0 {
		trusted = lua.NewState()
		configureTrustedPackagePath(trusted, trustedDirs)
		vm.addState(trusted)
	}
	for _, source := range vm.sources {
		if source.Trust == LuaTrustRestricted {
			L := newRestrictedState(source.Path)
			vm.runtimes[source.Path] = L
			vm.addState(L)
		} else {
			vm.runtimes[source.Path] = trusted
		}
	}
	if vm.state == nil {
		// Preserve the historical non-nil State contract for an empty source list.
		vm.addState(lua.NewState())
	}
	return vm
}

func (vm *LuaVM) addState(L *lua.LState) {
	if vm.state == nil {
		vm.state = L
	}
	vm.states = append(vm.states, L)
}

func configureTrustedPackagePath(L *lua.LState, dirs []string) {
	pkg, ok := L.GetGlobal("package").(*lua.LTable)
	if !ok {
		return
	}
	currentPath := lua.LVAsString(pkg.RawGetString("path"))
	paths := make([]string, 0, len(dirs)*2+1)
	for _, dir := range dirs {
		paths = append(paths, filepath.Join(dir, "?.lua"), filepath.Join(dir, "?/init.lua"))
	}
	if currentPath != "" {
		paths = append(paths, currentPath)
	}
	pkg.RawSetString("path", lua.LString(strings.Join(paths, ";")))
}

func newRestrictedState(root string) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	lua.OpenBase(L)
	lua.OpenTable(L)
	lua.OpenString(L)
	lua.OpenMath(L)
	lua.OpenCoroutine(L)
	for _, name := range []string{
		"collectgarbage", "dofile", "getfenv", "load", "loadfile", "loadstring",
		"module", "newproxy", "setfenv", "_printregs",
	} {
		L.SetGlobal(name, lua.LNil)
	}
	installRestrictedRequire(L, root)
	var printMu sync.Mutex
	printWindow := time.Now()
	printCount := 0
	printSuppressed := 0
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		parts := make([]string, 0, L.GetTop())
		for i := 1; i <= L.GetTop(); i++ {
			parts = append(parts, L.ToStringMeta(L.Get(i)).String())
		}
		message := strings.Join(parts, "\t")
		if len(message) > 2048 {
			message = message[:2048] + "…"
		}
		printMu.Lock()
		now := time.Now()
		if now.Sub(printWindow) >= time.Second {
			if printSuppressed > 0 {
				log.Printf("[LUA:%s] suppressed %d print messages", filepath.Base(root), printSuppressed)
			}
			printWindow = now
			printCount = 0
			printSuppressed = 0
		}
		if printCount >= 20 {
			printSuppressed++
			printMu.Unlock()
			return 0
		}
		printCount++
		log.Printf("[LUA:%s] %s", filepath.Base(root), message)
		printMu.Unlock()
		return 0
	}))
	return L
}

func installRestrictedRequire(L *lua.LState, root string) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = filepath.Clean(root)
	}
	cache := make(map[string]lua.LValue)
	loading := make(map[string]bool)
	L.SetGlobal("require", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if !validRestrictedModuleName(name) {
			L.RaiseError("restricted require denied module %q", name)
			return 0
		}
		if value, ok := cache[name]; ok {
			L.Push(value)
			return 1
		}
		if loading[name] {
			L.RaiseError("circular require of %q", name)
			return 0
		}
		relative := filepath.FromSlash(strings.ReplaceAll(name, ".", "/"))
		candidates := []string{filepath.Join(resolvedRoot, relative+".lua"), filepath.Join(resolvedRoot, relative, "init.lua")}
		modulePath := ""
		for _, candidate := range candidates {
			resolved, resolveErr := filepath.EvalSymlinks(candidate)
			if resolveErr != nil {
				continue
			}
			rel, relErr := filepath.Rel(resolvedRoot, resolved)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			if info, statErr := os.Stat(resolved); statErr == nil && !info.IsDir() {
				modulePath = resolved
				break
			}
		}
		if modulePath == "" {
			L.RaiseError("restricted module %q not found in source", name)
			return 0
		}
		fn, loadErr := L.LoadFile(modulePath)
		if loadErr != nil {
			L.RaiseError("loading restricted module %q: %v", name, loadErr)
			return 0
		}
		loading[name] = true
		callErr := callLua(L, lua.P{Fn: fn, NRet: 1, Protect: true})
		delete(loading, name)
		if callErr != nil {
			L.RaiseError("loading restricted module %q: %v", name, callErr)
			return 0
		}
		value := L.Get(-1)
		L.Pop(1)
		if value == lua.LNil {
			value = lua.LTrue
		}
		cache[name] = value
		L.Push(value)
		return 1
	}))
}

func validRestrictedModuleName(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if part == "" {
			return false
		}
		for i, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9')) {
				return false
			}
		}
	}
	return true
}

func resolveContainedFile(root, path string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes restricted source")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory")
	}
	return resolvedPath, nil
}

// Close shuts down the Lua state.
func (vm *LuaVM) Close() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	for _, L := range vm.states {
		L.Close()
	}
	vm.states = nil
	vm.state = nil
}

// LoadModes scans all script directories for .lua files, loads each one, and
// registers the returned mode table. Files are loaded in sorted order per
// directory for determinism. Later directories can override earlier ones.
func (vm *LuaVM) LoadModes() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, source := range vm.sources {
		dir := source.Path
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

			mode, err := vm.loadModeFile(name, path, vm.runtimes[source.Path], source)
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

// ResolveModeName returns the canonical (stored) name of the loaded mode whose
// name matches `name` case-insensitively, and whether such a mode exists. An
// exact match wins immediately. If several names differ only in case (highly
// unlikely), the match is arbitrary.
func (vm *LuaVM) ResolveModeName(name string) (string, bool) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if _, ok := vm.modes[name]; ok {
		return name, true
	}
	lower := strings.ToLower(name)
	for stored := range vm.modes {
		if strings.ToLower(stored) == lower {
			return stored, true
		}
	}
	return "", false
}

// ReloadMode re-reads a single mode file from disk and replaces the loaded
// mode. Shared libraries are uncached first so require() picks up changes.
// The matcher cache is cleared since patterns may have changed.
func (vm *LuaVM) ReloadMode(name string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.clearLibCache()

	for _, source := range vm.sources {
		dir := source.Path
		path := filepath.Join(dir, name+".lua")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		mode, err := vm.loadModeFile(name, path, vm.runtimes[source.Path], source)
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
	dirs := make([]string, 0, len(vm.sources))
	for _, source := range vm.sources {
		dirs = append(dirs, source.Path)
	}
	return dirs
}

func (vm *LuaVM) Sources() []LuaSource {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return append([]LuaSource(nil), vm.sources...)
}

func (vm *LuaVM) States() []*lua.LState {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return append([]*lua.LState(nil), vm.states...)
}

// clearLibCache removes all entries from Lua's package.loaded table
// so that subsequent require() calls re-read files from disk.
func (vm *LuaVM) clearLibCache() {
	for _, L := range vm.states {
		vm.clearStateLibCache(L)
	}
}

func (vm *LuaVM) clearStateLibCache(L *lua.LState) {
	pkg := L.GetGlobal("package")
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

// ModeSpecs returns the declared metadata for every loaded mode, sorted by name.
func (vm *LuaVM) ModeSpecs() []ModeSpec {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	specs := make([]ModeSpec, 0, len(vm.modes))
	for name, mode := range vm.modes {
		specs = append(specs, ModeSpec{
			Name:   name,
			Usage:  mode.Usage,
			Desc:   mode.Desc,
			Chains: mode.Chains,
			Hidden: mode.Hidden,
		})
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
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
func (vm *LuaVM) loadModeFile(name, path string, L *lua.LState, source LuaSource) (*LuaMode, error) {
	if source.Trust == LuaTrustRestricted {
		resolved, err := resolveContainedFile(source.Path, path)
		if err != nil {
			return nil, fmt.Errorf("restricted mode path %s: %w", path, err)
		}
		path = resolved
	}
	// Snapshot the stack depth so we can (a) detect a file that returns nothing
	// and (b) always restore the stack afterward, leaving it clean for the next
	// file regardless of how many values this one returned.
	base := L.GetTop()
	fn, err := L.LoadFile(path)
	if err != nil {
		L.SetTop(base)
		return nil, fmt.Errorf("executing %s: %w", path, err)
	}
	if err := callLua(L, lua.P{Fn: fn, NRet: lua.MultRet, Protect: true}); err != nil {
		L.SetTop(base)
		return nil, fmt.Errorf("executing %s: %w", path, err)
	}

	// A file with no `return M` leaves the stack unchanged. Popping in that case
	// underflows gopher-lua's register stack and panics (uncaught), so guard it
	// and report a skippable error instead of crashing the client.
	if L.GetTop() <= base {
		L.SetTop(base)
		return nil, fmt.Errorf("%s: mode file returned no value (did you forget `return M`?)", path)
	}

	// Take the first returned value (the mode table); `return M, extra` leaves
	// the extras above it. SetTop(base) then drops everything this file pushed.
	ret := L.Get(base + 1)
	L.SetTop(base)

	tbl, ok := ret.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s: expected table return, got %s", path, ret.Type())
	}

	mode := &LuaMode{
		Name: name, state: L, SourcePath: source.Path, Trust: source.Trust,
	}

	// Extract the declared metadata (all optional). A field of the wrong type is
	// treated as undeclared rather than failing the load: metadata is
	// descriptive, and a typo in it must never cost the player a working mode.
	if s, ok := tbl.RawGetString("usage").(lua.LString); ok {
		mode.Usage = string(s)
	}
	if s, ok := tbl.RawGetString("desc").(lua.LString); ok {
		mode.Desc = string(s)
	}
	if b, ok := tbl.RawGetString("chains").(lua.LBool); ok {
		mode.Chains = bool(b)
	}
	if b, ok := tbl.RawGetString("hidden").(lua.LBool); ok {
		mode.Hidden = bool(b)
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
