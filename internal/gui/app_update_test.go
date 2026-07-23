package gui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

func newTestApp(t *testing.T) *GuiApp {
	t.Helper()
	deps := &Deps{
		Config:     config.Defaults(),
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
		Version:    "0.2.0",
	}
	return NewGuiApp(deps, &captureEmitter{})
}

func TestSetInputSpellcheck_Persists(t *testing.T) {
	a := newTestApp(t)
	if !a.cfg().UI.InputSpellcheck {
		t.Fatal("spellcheck should default on")
	}
	if err := a.SetInputSpellcheck(false); err != nil {
		t.Fatalf("SetInputSpellcheck: %v", err)
	}
	got, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.UI.InputSpellcheck {
		t.Error("persisted config should have input_spellcheck=false")
	}
}

func TestSetUpdateCheck_Persists(t *testing.T) {
	a := newTestApp(t)
	if err := a.SetUpdateCheck(false); err != nil {
		t.Fatalf("SetUpdateCheck: %v", err)
	}
	got, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Updates.Check {
		t.Error("persisted config should have updates.check=false")
	}
}

func TestSetRetainAppLogs_PersistsForNextStartup(t *testing.T) {
	a := newTestApp(t)
	if a.cfg().Logging.App.Retain {
		t.Fatal("application-log retention should default off")
	}
	if err := a.SetRetainAppLogs(true); err != nil {
		t.Fatalf("SetRetainAppLogs: %v", err)
	}
	got, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.Logging.App.Retain {
		t.Error("persisted config should have logging.app.retain=true")
	}
}

func TestCheckForUpdate_DisabledSkipsNetwork(t *testing.T) {
	a := newTestApp(t)
	a.cfg().Updates.Check = false

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	old := updateEndpoint
	updateEndpoint = srv.URL
	defer func() { updateEndpoint = old }()

	info := a.CheckForUpdate()
	if called {
		t.Error("disabled update check must not hit the network")
	}
	if info.Available {
		t.Errorf("disabled check should report no update: %+v", info)
	}
	if info.Current != "0.2.0" {
		t.Errorf("current = %q, want 0.2.0", info.Current)
	}
}

func TestCheckForUpdate_Available(t *testing.T) {
	a := newTestApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.9.9","html_url":"https://example.com/r"}`))
	}))
	defer srv.Close()
	old := updateEndpoint
	updateEndpoint = srv.URL
	defer func() { updateEndpoint = old }()

	info := a.CheckForUpdate()
	if !info.Available || info.Latest != "0.9.9" {
		t.Fatalf("info = %+v", info)
	}
}

func TestCheckForUpdate_FailureReportsNoUpdate(t *testing.T) {
	a := newTestApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	old := updateEndpoint
	updateEndpoint = srv.URL
	defer func() { updateEndpoint = old }()

	info := a.CheckForUpdate()
	if info.Available {
		t.Errorf("failed check must not report an update: %+v", info)
	}
}
