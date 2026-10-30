package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/scriptmanager"
)

type guiScriptGit struct {
	info        scriptmanager.RemoteInfo
	inspected   []string
	mode        string
	checkoutErr error
}

func (g *guiScriptGit) Inspect(_ context.Context, rawURL string) (scriptmanager.RemoteInfo, error) {
	g.inspected = append(g.inspected, rawURL)
	return g.info, nil
}

func (g *guiScriptGit) Checkout(_ context.Context, req scriptmanager.CheckoutRequest) (string, error) {
	if g.checkoutErr != nil {
		return "", g.checkoutErr
	}
	mode := g.mode
	if mode == "" {
		mode = "return {}\n"
	}
	if err := os.WriteFile(filepath.Join(req.Destination, "mode.lua"), []byte(mode), 0o644); err != nil {
		return "", err
	}
	return g.info.Tags[req.Ref], nil
}

func newScriptManagerTestApp(t *testing.T) (*GuiApp, *guiScriptGit) {
	t.Helper()
	dir := t.TempDir()
	backend := &guiScriptGit{info: scriptmanager.RemoteInfo{Tags: map[string]string{"v0.2.4": "commit-024"}}}
	cfg := config.Defaults()
	gameClient, err := client.NewClient(cfg, nil, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(gameClient.Engine.Close)
	deps := &Deps{
		Client:        gameClient,
		Config:        cfg,
		ConfigPath:    filepath.Join(dir, "config.yaml"),
		ScriptManager: scriptmanager.NewWithBackend(filepath.Join(dir, "managed-scripts"), backend),
	}
	return NewGuiApp(deps, &captureEmitter{}), backend
}

func TestManagedScriptInstallIsDisabledRestrictedAndSuggestedChecksAtStartup(t *testing.T) {
	a, _ := newScriptManagerTestApp(t)
	source, err := a.InstallManagedScript(scriptmanager.SuggestedRepositoryURL)
	if err != nil {
		t.Fatal(err)
	}
	if source.Enabled || source.Trust != config.ScriptTrustRestricted || !source.CheckOnStartup {
		t.Fatalf("installed source = %+v", source)
	}
	state := a.GetScriptManager()
	if !state.SuggestedInstalled || len(state.Managed) != 1 || state.Managed[0].ID != source.ID {
		t.Fatalf("manager state = %+v", state)
	}
	loaded, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ManagedScripts) != 1 || loaded.ManagedScripts[0].ID != source.ID {
		t.Fatalf("saved sources = %+v", loaded.ManagedScripts)
	}
	if _, err := a.InstallManagedScript(scriptmanager.SuggestedRepositoryURL); err == nil {
		t.Fatal("duplicate install succeeded")
	}
}

func TestManagedScriptEnableLoadsRestrictedSourceAndDisableUnloadsIt(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { desc = os and "trusted" or "restricted", reactions = {} }`
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(source.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if !a.client().Engine.HasMode("mode") || !a.GetScriptManager().Managed[0].Enabled {
		t.Fatal("enabled managed mode was not loaded")
	}
	if got := modeDescription(a, "mode"); got != "restricted" {
		t.Fatalf("enabled source description = %q, want restricted", got)
	}
	if err := a.SetManagedScriptTrust(source.ID, config.ScriptTrustTrusted, false); err != nil {
		t.Fatal(err)
	}
	if got := modeDescription(a, "mode"); got != "trusted" {
		t.Fatalf("trusted source description = %q, want trusted", got)
	}
	if err := a.SetManagedScriptEnabled(source.ID, false, false); err != nil {
		t.Fatal(err)
	}
	if a.client().Engine.HasMode("mode") || a.GetScriptManager().Managed[0].Enabled {
		t.Fatal("disabled managed mode remains loaded")
	}
}

func TestManagedScriptRuntimeChangeRequiresStoppingActiveMode(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { on_start = function() end, reactions = {} }`
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(source.ID, true, false); err != nil {
		t.Fatal(err)
	}
	a.client().Engine.SetMode("mode", nil)
	if err := a.SetManagedScriptEnabled(source.ID, false, false); err == nil {
		t.Fatal("active mode source changed without explicit stop")
	}
	if a.client().Engine.CurrentMode() != "mode" || !a.GetScriptManager().Managed[0].Enabled {
		t.Fatal("rejected change modified the active runtime")
	}
	if err := a.SetManagedScriptEnabled(source.ID, false, true); err != nil {
		t.Fatal(err)
	}
	if a.client().Engine.CurrentMode() != "disable" || a.GetScriptManager().Managed[0].Enabled {
		t.Fatal("confirmed change did not stop automation and disable the source")
	}
}

func TestManagedScriptReorderReloadsSourcePrecedence(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { desc = "first", reactions = {} }`
	first, err := a.InstallManagedScript("https://example.test/first.git")
	if err != nil {
		t.Fatal(err)
	}
	backend.mode = `return { desc = "second", reactions = {} }`
	second, err := a.InstallManagedScript("https://example.test/second.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(first.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(second.ID, true, false); err != nil {
		t.Fatal(err)
	}
	if got := modeDescription(a, "mode"); got != "second" {
		t.Fatalf("initial source precedence = %q, want second", got)
	}
	if err := a.ReorderManagedScripts([]string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	if got := modeDescription(a, "mode"); got != "first" {
		t.Fatalf("reordered source precedence = %q, want first", got)
	}
}

func modeDescription(a *GuiApp, name string) string {
	for _, spec := range a.client().Engine.ModeSpecs() {
		if spec.Name == name {
			return spec.Desc
		}
	}
	return ""
}

func TestManagedScriptCheckUpdatePreferenceAndRemove(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	backend.info.Tags["v0.3.0"] = "commit-030"
	status, err := a.CheckManagedScript(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Available || !status.Updated || status.Version != "v0.3.0" {
		t.Fatalf("status = %+v", status)
	}
	if err := a.SetManagedScriptStartupCheck(source.ID, true); err != nil {
		t.Fatal(err)
	}
	if !a.GetScriptManager().Managed[0].CheckOnStartup {
		t.Fatal("startup check preference was not saved")
	}
	if err := a.RemoveManagedScript(source.ID); err != nil {
		t.Fatal(err)
	}
	if len(a.GetScriptManager().Managed) != 0 {
		t.Fatal("removed source remains in state")
	}
	if _, err := os.Stat(a.deps.ScriptManager.Path(source.ID)); !os.IsNotExist(err) {
		t.Fatalf("managed checkout remains: %v", err)
	}
}

func TestManagedScriptCheckUpdatesEnabledSourceAndRestoresIt(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { desc = "old", reactions = {} }`
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(source.ID, true, false); err != nil {
		t.Fatal(err)
	}

	backend.info.Tags["v0.3.0"] = "commit-030"
	backend.mode = `return { desc = "new", reactions = {} }`
	status, err := a.CheckManagedScript(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Available || !status.Updated {
		t.Fatalf("applied update status = %+v", status)
	}
	state := a.GetScriptManager().Managed[0]
	if !state.Enabled {
		t.Fatal("automatic update left the source disabled")
	}
	if state.InstalledVersion != "v0.3.0" || state.InstalledCommit != "commit-030" {
		t.Fatalf("updated source = %+v", state)
	}
	if got := modeDescription(a, "mode"); got != "new" {
		t.Fatalf("reloaded source description = %q, want new", got)
	}

	loaded, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.ManagedScripts) != 1 || !loaded.ManagedScripts[0].Enabled || loaded.ManagedScripts[0].InstalledVersion != "v0.3.0" {
		t.Fatalf("saved source = %+v", loaded.ManagedScripts)
	}
}

func TestManagedScriptCheckRefusesAutomaticUpdateWhileModeActive(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { desc = "old", on_start = function() end, reactions = {} }`
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(source.ID, true, false); err != nil {
		t.Fatal(err)
	}
	a.client().Engine.SetMode("mode", nil)

	backend.info.Tags["v0.3.0"] = "commit-030"
	backend.mode = `return { desc = "new", reactions = {} }`
	status, err := a.CheckManagedScript(source.ID)
	if err == nil {
		t.Fatal("automatic update succeeded while a Lua mode was active")
	}
	if !status.Available {
		t.Fatalf("blocked update did not retain its available status: %+v", status)
	}
	state := a.GetScriptManager().Managed[0]
	if !state.Enabled || state.InstalledVersion != "v0.2.4" || state.InstalledCommit != "commit-024" {
		t.Fatalf("blocked update changed the source: %+v", state)
	}
	if a.client().Engine.CurrentMode() != "mode" || modeDescription(a, "mode") != "old" {
		t.Fatal("blocked update changed the active runtime")
	}
}

func TestManagedScriptCheckRollsBackAndRestoresEnabledSource(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	backend.mode = `return { desc = "old", reactions = {} }`
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SetManagedScriptEnabled(source.ID, true, false); err != nil {
		t.Fatal(err)
	}

	backend.info.Tags["v0.3.0"] = "commit-030"
	backend.checkoutErr = errors.New("download interrupted")
	status, err := a.CheckManagedScript(source.ID)
	if err == nil {
		t.Fatal("failed update succeeded")
	}
	if !status.Available || status.Updated || status.Error == "" {
		t.Fatalf("failed update status = %+v", status)
	}
	state := a.GetScriptManager().Managed[0]
	if !state.Enabled || state.InstalledVersion != "v0.2.4" || state.InstalledCommit != "commit-024" {
		t.Fatalf("failed update changed the source: %+v", state)
	}
	if got := modeDescription(a, "mode"); got != "old" {
		t.Fatalf("restored source description = %q, want old", got)
	}
}

func TestStartupScriptChecksHonorPerSourcePreference(t *testing.T) {
	a, backend := newScriptManagerTestApp(t)
	source, err := a.InstallManagedScript("https://example.test/scripts.git")
	if err != nil {
		t.Fatal(err)
	}
	backend.info.Tags["v0.3.0"] = "commit-030"
	inspections := len(backend.inspected)
	a.checkStartupScriptSources(context.Background())
	if len(backend.inspected) != inspections {
		t.Fatal("startup checked an opted-out source")
	}
	if err := a.SetManagedScriptStartupCheck(source.ID, true); err != nil {
		t.Fatal(err)
	}
	a.checkStartupScriptSources(context.Background())
	if len(backend.inspected) != inspections+1 {
		t.Fatalf("startup inspections = %d, want %d", len(backend.inspected), inspections+1)
	}
	status := a.GetScriptManager().Managed[0].Status
	if !status.Available || status.Version != "v0.3.0" {
		t.Fatalf("startup status = %+v", status)
	}
}
