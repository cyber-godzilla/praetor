package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
)

// stubCreds satisfies session.CredentialStore for tests that reach
// GetInitState, which lists accounts before assembling the payload.
type stubCreds struct{}

func (stubCreds) Descriptor() session.CredentialStoreDescriptor {
	return session.CredentialStoreDescriptor{Backend: "test", CanStore: true}
}
func (stubCreds) ListAccounts() ([]string, error)     { return nil, nil }
func (stubCreds) GetAccount(string) (string, error)   { return "", nil }
func (stubCreds) SetAccount(string, string) error     { return nil }
func (stubCreds) RepairAccounts(string, string) error { return nil }
func (stubCreds) RemoveAccount(string) error          { return nil }

// newModeSpecApp builds a GuiApp over a real client whose engine has loaded a
// script directory, so the metadata under test travels the same path it does in
// the app: Lua file -> loadModeFile -> Engine.ModeSpecs -> facade.
func newModeSpecApp(t *testing.T, modes map[string]string) *GuiApp {
	t.Helper()
	dir := t.TempDir()
	return newModeSpecAppInDir(t, dir, modes)
}

func newModeSpecAppInDir(t *testing.T, dir string, modes map[string]string) *GuiApp {
	t.Helper()
	for name, body := range modes {
		path := filepath.Join(dir, name+".lua")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	c, err := client.NewClient(config.Defaults(), []string{dir}, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Engine.Close)

	deps := &Deps{Config: config.Defaults(), Client: c, Creds: stubCreds{}, Version: "0.2.0"}
	return NewGuiApp(deps, &captureEmitter{})
}

func TestReloadScriptsRescansChangedAddedAndRemovedModeSpecs(t *testing.T) {
	dir := t.TempDir()
	a := newModeSpecAppInDir(t, dir, map[string]string{
		"alpha": `
local M = {}
M.usage = '<old>'
M.desc = 'old description'
M.reactions = {}
return M
`,
		"removed": `
local M = {}
M.desc = 'remove me'
M.reactions = {}
return M
`,
	})

	if err := os.WriteFile(filepath.Join(dir, "alpha.lua"), []byte(`
local M = {}
M.usage = '<new>'
M.desc = 'new description'
M.chains = true
M.reactions = {}
return M
`), 0o644); err != nil {
		t.Fatalf("rewrite alpha: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "removed.lua")); err != nil {
		t.Fatalf("remove old mode: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "beta.lua"), []byte(`
local M = {}
M.usage = '[value]'
M.desc = 'new mode'
M.reactions = {}
return M
`), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	if err := a.ReloadScripts(); err != nil {
		t.Fatalf("ReloadScripts: %v", err)
	}
	specs := a.ModeSpecs()
	if len(specs) != 2 {
		t.Fatalf("ModeSpecs after reload = %+v, want alpha and beta", specs)
	}
	if got := specs[0]; got.Name != "alpha" || got.Usage != "<new>" ||
		got.Desc != "new description" || !got.Chains {
		t.Errorf("updated alpha spec = %+v", got)
	}
	if got := specs[1]; got.Name != "beta" || got.Usage != "[value]" ||
		got.Desc != "new mode" || got.Chains {
		t.Errorf("added beta spec = %+v", got)
	}
}

func TestGetInitState_CarriesModeSpecs(t *testing.T) {
	a := newModeSpecApp(t, map[string]string{
		"loot": `
local M = {}
M.usage = '<item> [corpse#]'
M.desc = 'Take an item from every corpse'
M.chains = true
M.reactions = {}
return M
`,
	})

	specs := a.GetInitState().ModeSpecs
	if len(specs) != 1 {
		t.Fatalf("InitState.ModeSpecs = %+v, want 1 entry", specs)
	}
	got := specs[0]
	if got.Name != "loot" || got.Usage != "<item> [corpse#]" ||
		got.Desc != "Take an item from every corpse" || !got.Chains {
		t.Errorf("InitState.ModeSpecs[0] = %+v, want the declared metadata", got)
	}
}

func TestModeSpecs_BindingAgreesWithInitState(t *testing.T) {
	// The binding exists so the frontend can refresh after a script reload; it
	// must not drift from what the initial payload reported.
	a := newModeSpecApp(t, map[string]string{
		"alpha": `
local M = {}
M.desc = 'first'
M.reactions = {}
return M
`,
		"lib_util": `
local M = {}
M.desc = 'a library, not a mode'
M.reactions = {}
return M
`,
	})

	binding := a.ModeSpecs()
	init := a.GetInitState().ModeSpecs

	if len(binding) != len(init) {
		t.Fatalf("ModeSpecs() = %+v, InitState.ModeSpecs = %+v, want equal", binding, init)
	}
	for i := range binding {
		if binding[i] != init[i] {
			t.Errorf("index %d: binding %+v != init %+v", i, binding[i], init[i])
		}
	}
	for _, s := range binding {
		if s.Name == "lib_util" {
			t.Errorf("ModeSpecs() = %+v, should exclude lib_-prefixed modes", binding)
		}
	}
}

// The frontend reads this payload as JSON over the Wails bridge, so the Go
// struct being right is not sufficient — the field has to survive marshalling
// under the name the frontend looks for.
func TestInitStateJSON_CarriesModeSpecs(t *testing.T) {
	a := newModeSpecApp(t, map[string]string{
		"loot": `
local M = {}
M.usage = '<item> [corpse#]'
M.desc = 'Take an item from every corpse'
M.chains = true
M.reactions = {}
return M
`,
	})

	blob, err := json.Marshal(a.GetInitState())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var wire struct {
		ModeSpecs []struct {
			Name   string `json:"name"`
			Usage  string `json:"usage"`
			Desc   string `json:"desc"`
			Chains bool   `json:"chains"`
		} `json:"modeSpecs"`
	}
	if err := json.Unmarshal(blob, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wire.ModeSpecs) != 1 {
		t.Fatalf("modeSpecs in %s = %d entries, want 1", blob, len(wire.ModeSpecs))
	}
	got := wire.ModeSpecs[0]
	if got.Name != "loot" || got.Usage != "<item> [corpse#]" ||
		got.Desc != "Take an item from every corpse" || !got.Chains {
		t.Errorf("modeSpecs[0] = %+v, want the declared metadata", got)
	}
}
