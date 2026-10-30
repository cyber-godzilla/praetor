package main

import (
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/engine"
)

func TestBuildScriptSourcesKeepsEnabledManagedSources(t *testing.T) {
	configDir := t.TempDir()
	sources := buildScriptSources([]string{"/local"}, []config.ManagedScriptSourceConfig{
		{ID: "off", Enabled: false, Trust: config.ScriptTrustRestricted},
		{ID: "safe", Enabled: true, Trust: config.ScriptTrustRestricted},
		{ID: "legacy", Enabled: true, Trust: config.ScriptTrustTrusted},
	}, configDir)
	want := []engine.LuaSource{
		{Path: "/local", Trust: engine.LuaTrustTrusted},
		{Path: filepath.Join(configDir, "managed-scripts", "safe"), Trust: engine.LuaTrustRestricted},
		{Path: filepath.Join(configDir, "managed-scripts", "legacy"), Trust: engine.LuaTrustTrusted},
	}
	if len(sources) != len(want) {
		t.Fatalf("sources = %#v, want %#v", sources, want)
	}
	for index := range want {
		if sources[index] != want[index] {
			t.Fatalf("sources[%d] = %#v, want %#v", index, sources[index], want[index])
		}
	}
}
