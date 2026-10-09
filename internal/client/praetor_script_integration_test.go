package client

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
)

// newLuaPraetorScriptClient loads the supplied real Lua modes through
// NewClient. That matters here: NewClient is where praetor_script() is wired to
// the same SendInput pipeline used by typed input and action-set buttons.
func newLuaPraetorScriptClient(t *testing.T, modes map[string]string) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range modes {
		path := filepath.Join(dir, name+".lua")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	c, err := NewClient(config.Defaults(), []string{dir}, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)
	return c, dir
}

func TestLuaPraetorScriptUsesFullTypedInputPipeline(t *testing.T) {
	c, _ := newLuaPraetorScriptClient(t, map[string]string{
		"qa": `
local M = {}
M.on_start = function(args)
    praetor_script([[prepare ${target:fallback};;$(wait 0.01);;$(repeat "climb ${target:fallback}" until "at top" max 2);;$(notify "Lua QA" "done");;look]])
end
M.reactions = {}
return M
`,
	})
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	connectTestSession(t, c, wsURL)
	c.SetInputVariables(map[string]string{"target": "arch"})
	c.SetSemicolonDelay(10 * time.Millisecond)
	notified := make(chan string, 1)
	c.desktopNotify = func(title, message string, _ bool) {
		notified <- title + ":" + message
	}

	c.Engine.SetMode("qa", nil)
	receiveCommand(t, received, "prepare arch")
	receiveCommand(t, received, "climb arch")
	c.processLine("You arrive at top.")
	select {
	case got := <-notified:
		if got != "Lua QA:done" {
			t.Fatalf("notification = %q, want %q", got, "Lua QA:done")
		}
	case <-time.After(time.Second):
		t.Fatal("Lua-originated PraetorScript did not send its titled notification")
	}
	receiveCommand(t, received, "look")
	waitForInputChainInactive(t, c)
}

func TestLuaPraetorScriptSupportsUnbusyAndBoundedWaitFor(t *testing.T) {
	c, _ := newLuaPraetorScriptClient(t, map[string]string{
		"qa": `
local M = {}
M.on_start = function(args)
    praetor_script([[stand&&search chest;;$(wait-for "You find a key" timeout 1);;get key]])
end
M.reactions = {}
return M
`,
	})
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(10 * time.Millisecond)
	c.SetUnbusyDelay(0)

	c.Engine.SetMode("qa", nil)
	receiveCommand(t, received, "stand")
	receiveNoCommand(t, received, 40*time.Millisecond)
	c.processLine("You stop walking.")
	receiveCommand(t, received, "search chest")
	receiveNoCommand(t, received, 40*time.Millisecond)
	c.processLine("You find a key in the chest.")
	receiveCommand(t, received, "get key")
	waitForInputChainInactive(t, c)
}

func TestLuaPraetorScriptSupportsCountedRepeat(t *testing.T) {
	c, _ := newLuaPraetorScriptClient(t, map[string]string{
		"qa": `
local M = {}
M.on_start = function(args)
    praetor_script([[$(repeat "search" count ${tries:2});;look]])
end
M.reactions = {}
return M
`,
	})
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	connectTestSession(t, c, wsURL)
	c.SetSemicolonDelay(0)
	c.SetUnbusyDelay(0)

	c.Engine.SetMode("qa", nil)
	receiveCommand(t, received, "search")
	c.processLine("You are no longer busy.")
	receiveCommand(t, received, "search")
	c.processLine("You stop walking.")
	receiveCommand(t, received, "look")
	waitForInputChainInactive(t, c)
}

func TestLuaPraetorScriptCanRunLocalModeCommandWithoutDeadlock(t *testing.T) {
	c, _ := newLuaPraetorScriptClient(t, map[string]string{
		"source": `
local M = {}
M.on_start = function(args)
    praetor_script([[/mode target alpha "2 sack"]])
end
M.reactions = {}
return M
`,
		"target": `
local M = {}
M.on_start = function(args)
    praetor_script("args " .. table.concat(args, "|"))
end
M.reactions = {}
return M
`,
	})
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	connectTestSession(t, c, wsURL)

	c.Engine.SetMode("source", nil)
	receiveCommand(t, received, "args alpha|2 sack")
	if got := c.Engine.CurrentMode(); got != "target" {
		t.Fatalf("CurrentMode = %q, want target", got)
	}
}

func TestLuaPraetorScriptHandlerSurvivesScriptReload(t *testing.T) {
	c, dir := newLuaPraetorScriptClient(t, map[string]string{
		"qa": praetorScriptMode("before"),
	})
	srv, wsURL, received := newRecordingServer(t)
	defer srv.Close()
	connectTestSession(t, c, wsURL)

	c.Engine.SetMode("qa", nil)
	receiveCommand(t, received, "before")

	path := filepath.Join(dir, "qa.lua")
	if err := os.WriteFile(path, []byte(praetorScriptMode("after")), 0o644); err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
	if err := c.Engine.ReloadAllModes(); err != nil {
		t.Fatalf("ReloadAllModes: %v", err)
	}
	c.Engine.SetMode("disable", nil)
	c.Engine.SetMode("qa", nil)
	receiveCommand(t, received, "after")
}

func praetorScriptMode(command string) string {
	return fmt.Sprintf(`
local M = {}
M.on_start = function(args)
    praetor_script(%q)
end
M.reactions = {}
return M
`, command)
}
