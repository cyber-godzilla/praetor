package engine

import (
	"os"
	"path/filepath"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// SentCommand records a send() call.
type SentCommand struct {
	Command string
	DelayMs int
}

// ModeChangeRequest records a set_mode() call.
type ModeChangeRequest struct {
	Mode string
	Args []string
}

// NotificationRequest records a notify() call.
type NotificationRequest struct {
	Title   string
	Message string
}

// BridgeSink implements BridgeCallbacks and captures all calls for assertions.
type BridgeSink struct {
	Commands      []SentCommand
	ModeChanges   []ModeChangeRequest
	Notifications []NotificationRequest
	Logs          []string
	Metrics       map[string]int
}

func (s *BridgeSink) OnSend(command string, delayMs int) {
	s.Commands = append(s.Commands, SentCommand{Command: command, DelayMs: delayMs})
}

func (s *BridgeSink) OnSetMode(mode string, args []string) {
	s.ModeChanges = append(s.ModeChanges, ModeChangeRequest{Mode: mode, Args: args})
}

func (s *BridgeSink) OnNotify(title, message string) {
	s.Notifications = append(s.Notifications, NotificationRequest{Title: title, Message: message})
}

func (s *BridgeSink) OnLog(message string) {
	s.Logs = append(s.Logs, message)
}

func (s *BridgeSink) OnMetricsTrack(key, label string) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]int)
	}
	if _, ok := s.Metrics[key]; !ok {
		s.Metrics[key] = 0
	}
}

func (s *BridgeSink) OnMetricsInc(key string) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]int)
	}
	s.Metrics[key]++
}

func (s *BridgeSink) OnMetricsDec(key string) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]int)
	}
	s.Metrics[key]--
}

func (s *BridgeSink) OnMetricsSet(key string, value int) {
	if s.Metrics == nil {
		s.Metrics = make(map[string]int)
	}
	s.Metrics[key] = value
}

func (s *BridgeSink) OnMetricsGet(key string) int {
	if s.Metrics == nil {
		return 0
	}
	return s.Metrics[key]
}

func newTestBridge(t *testing.T) (*lua.LState, *BridgeSink, *StatusValues) {
	t.Helper()
	L := lua.NewState()
	sink := &BridgeSink{}
	status := &StatusValues{Health: 100, Fatigue: 50, Encumbrance: 25, Satiation: 75}
	RegisterBridge(L, sink, status)
	return L, sink, status
}

func TestBridge_SendWithoutDelay(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`send("look")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.Commands) != 1 {
		t.Fatalf("Commands len = %d, want 1", len(sink.Commands))
	}
	if sink.Commands[0].Command != "look" {
		t.Errorf("Command = %q, want 'look'", sink.Commands[0].Command)
	}
	if sink.Commands[0].DelayMs != 0 {
		t.Errorf("DelayMs = %d, want 0", sink.Commands[0].DelayMs)
	}
}

func TestBridge_SendWithDelay(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`send("attack gladiator", 500)`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.Commands) != 1 {
		t.Fatalf("Commands len = %d, want 1", len(sink.Commands))
	}
	if sink.Commands[0].Command != "attack gladiator" {
		t.Errorf("Command = %q, want 'attack gladiator'", sink.Commands[0].Command)
	}
	if sink.Commands[0].DelayMs != 500 {
		t.Errorf("DelayMs = %d, want 500", sink.Commands[0].DelayMs)
	}
}

func TestBridge_SetModeWithoutArgs(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`set_mode("macro")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.ModeChanges) != 1 {
		t.Fatalf("ModeChanges len = %d, want 1", len(sink.ModeChanges))
	}
	if sink.ModeChanges[0].Mode != "macro" {
		t.Errorf("Mode = %q, want 'macro'", sink.ModeChanges[0].Mode)
	}
	if len(sink.ModeChanges[0].Args) != 0 {
		t.Errorf("Args = %v, want empty", sink.ModeChanges[0].Args)
	}
}

func TestBridge_SetModeWithArgs(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`set_mode("loot", {"sword", "shield"})`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.ModeChanges) != 1 {
		t.Fatalf("ModeChanges len = %d, want 1", len(sink.ModeChanges))
	}
	if sink.ModeChanges[0].Mode != "loot" {
		t.Errorf("Mode = %q, want 'loot'", sink.ModeChanges[0].Mode)
	}
	if len(sink.ModeChanges[0].Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(sink.ModeChanges[0].Args))
	}
	if sink.ModeChanges[0].Args[0] != "sword" || sink.ModeChanges[0].Args[1] != "shield" {
		t.Errorf("Args = %v, want [sword, shield]", sink.ModeChanges[0].Args)
	}
}

func TestBridge_Notify(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`notify("Alert", "You were attacked!")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.Notifications) != 1 {
		t.Fatalf("Notifications len = %d, want 1", len(sink.Notifications))
	}
	if sink.Notifications[0].Title != "Alert" {
		t.Errorf("Title = %q, want 'Alert'", sink.Notifications[0].Title)
	}
	if sink.Notifications[0].Message != "You were attacked!" {
		t.Errorf("Message = %q, want 'You were attacked!'", sink.Notifications[0].Message)
	}
}

func TestBridge_Log(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`log("debug: entering macro mode")`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if len(sink.Logs) != 1 {
		t.Fatalf("Logs len = %d, want 1", len(sink.Logs))
	}
	if sink.Logs[0] != "debug: entering macro mode" {
		t.Errorf("Log = %q, want 'debug: entering macro mode'", sink.Logs[0])
	}
}

func TestBridge_RandomItem(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	// Test that random_item returns an element from the table
	err := L.DoString(`
		local items = {"a", "b", "c"}
		local result = random_item(items)
		assert(result == "a" or result == "b" or result == "c",
			"random_item should return element from set, got: " .. tostring(result))
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_RandomItemEmpty(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		local result = random_item({})
		assert(result == nil, "random_item on empty table should return nil")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_TimeNow(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		local ts = time.now()
		assert(type(ts) == "number", "time.now() should return number")
		assert(ts > 0, "time.now() should return positive number")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_TimeSince(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		local ts = time.now()
		local elapsed = time.since(ts)
		assert(type(elapsed) == "number", "time.since() should return number")
		assert(elapsed >= 0, "time.since() should be non-negative")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_StatusReadable(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		assert(status.health == 100, "health should be 100, got: " .. tostring(status.health))
		assert(status.fatigue == 50, "fatigue should be 50, got: " .. tostring(status.fatigue))
		assert(status.encumbrance == 25, "encumbrance should be 25, got: " .. tostring(status.encumbrance))
		assert(status.satiation == 75, "satiation should be 75, got: " .. tostring(status.satiation))
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_StatusUnknownField(t *testing.T) {
	L, _, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		assert(status.nonexistent == nil, "unknown status field should be nil")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}
}

func TestBridge_MetricsFunctions(t *testing.T) {
	L, sink, _ := newTestBridge(t)
	defer L.Close()

	err := L.DoString(`
		metrics.inc("kills")
		metrics.inc("kills")
		metrics.inc("crits")
		metrics.inc("actions")
		metrics.inc("actions")
		metrics.inc("actions")
	`)
	if err != nil {
		t.Fatalf("DoString error: %v", err)
	}

	if sink.Metrics["kills"] != 2 {
		t.Errorf("KillCount = %d, want 2", sink.Metrics["kills"])
	}
	if sink.Metrics["crits"] != 1 {
		t.Errorf("CritCount = %d, want 1", sink.Metrics["crits"])
	}
	if sink.Metrics["actions"] != 3 {
		t.Errorf("ActionCount = %d, want 3", sink.Metrics["actions"])
	}
}

func TestBridge_EndToEnd(t *testing.T) {
	// Full integration: load a real mode file, register bridge and state,
	// call on_start, then fire a reaction, verify commands and state.
	modesDir := t.TempDir()
	libDir := t.TempDir()

	err := os.WriteFile(filepath.Join(modesDir, "loot.lua"), []byte(`
local M = {}

M.on_start = function(args)
    state.set("item", args[1])
    state.set("corpse", 1)
    send("get " .. args[1] .. " from 1 corpse")
end

M.reactions = {
    {
        match = "You take",
        action = function()
            local corpse = state.get("corpse")
            local item = state.get("item")
            send("get " .. item .. " from " .. corpse .. " corpse")
        end,
    },
    {
        match = "You don't see",
        action = function()
            local corpse = state.get("corpse") + 1
            state.set("corpse", corpse)
            local item = state.get("item")
            send("get " .. item .. " from " .. corpse .. " corpse")
        end,
    },
}

return M
`), 0644)
	if err != nil {
		t.Fatalf("writing mode file: %v", err)
	}

	vm := NewLuaVM([]string{modesDir, libDir})
	defer vm.Close()

	L := vm.State()
	sink := &BridgeSink{}
	status := &StatusValues{}
	ms := NewModeState()

	RegisterBridge(L, sink, status)
	RegisterStateAPI(L, ms)

	err = vm.LoadModes()
	if err != nil {
		t.Fatalf("LoadModes() error: %v", err)
	}

	mode, ok := vm.GetMode("loot")
	if !ok {
		t.Fatal("GetMode('loot') not found")
	}

	// Call on_start with args
	argsTbl := L.NewTable()
	argsTbl.RawSetInt(1, lua.LString("sword"))

	err = L.CallByParam(lua.P{
		Fn:      mode.onStartRef,
		NRet:    0,
		Protect: true,
	}, argsTbl)
	if err != nil {
		t.Fatalf("on_start error: %v", err)
	}

	// Verify on_start sent initial command
	if len(sink.Commands) != 1 {
		t.Fatalf("on_start: Commands len = %d, want 1", len(sink.Commands))
	}
	if sink.Commands[0].Command != "get sword from 1 corpse" {
		t.Errorf("on_start command = %q, want 'get sword from 1 corpse'", sink.Commands[0].Command)
	}

	// Verify state was set
	itemVal, ok := ms.GetValue("item")
	if !ok || lua.LVAsString(itemVal) != "sword" {
		t.Errorf("state.get('item') = %v, want 'sword'", itemVal)
	}

	// Fire the "You take" reaction
	err = L.CallByParam(lua.P{
		Fn:      mode.Reactions[0].actionRef,
		NRet:    0,
		Protect: true,
	})
	if err != nil {
		t.Fatalf("reaction action error: %v", err)
	}

	if len(sink.Commands) != 2 {
		t.Fatalf("after reaction: Commands len = %d, want 2", len(sink.Commands))
	}
	if sink.Commands[1].Command != "get sword from 1 corpse" {
		t.Errorf("reaction command = %q, want 'get sword from 1 corpse'", sink.Commands[1].Command)
	}

	// Fire the "You don't see" reaction to increment corpse
	err = L.CallByParam(lua.P{
		Fn:      mode.Reactions[1].actionRef,
		NRet:    0,
		Protect: true,
	})
	if err != nil {
		t.Fatalf("reaction[1] action error: %v", err)
	}

	if len(sink.Commands) != 3 {
		t.Fatalf("after reaction[1]: Commands len = %d, want 3", len(sink.Commands))
	}
	if sink.Commands[2].Command != "get sword from 2 corpse" {
		t.Errorf("reaction[1] command = %q, want 'get sword from 2 corpse'", sink.Commands[2].Command)
	}

	// Verify corpse was incremented in state
	corpseVal, ok := ms.GetValue("corpse")
	if !ok {
		t.Fatal("state.get('corpse') not found")
	}
	if lua.LVAsNumber(corpseVal) != 2 {
		t.Errorf("state.get('corpse') = %v, want 2", corpseVal)
	}
}
