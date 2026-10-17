package gui

import (
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestSetViewLayoutPersistsLayoutAndCustomViewsAtomically(t *testing.T) {
	cfg := config.Defaults()
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewGuiApp(&Deps{Config: cfg, ConfigPath: path}, &captureEmitter{})

	customID := "1fc95886-1541-4db4-8a49-d30391439c65"
	custom := []config.CustomTabConfig{{ID: customID, Name: "Chat", Visible: true}}
	layout := *cfg.DefaultDisplayProfile()
	layout.Main = append([]config.DisplayComponentConfig(nil), layout.Main...)
	layout.Sidebars = append([]config.DisplaySidebarConfig(nil), layout.Sidebars...)
	for i := range layout.Sidebars {
		layout.Sidebars[i].Components = append([]config.DisplayComponentConfig(nil), layout.Sidebars[i].Components...)
	}
	layout.Sidebars[0].Side = "left"
	layout.Sidebars[1].Enabled = true
	layout.Sidebars[1].Components = append(layout.Sidebars[1].Components, config.DisplayComponentConfig{
		ComponentID: customID, SidebarHeight: 310,
	})
	if err := a.SetViewLayout(layout, custom); err != nil {
		t.Fatalf("SetViewLayout: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	loadedDisplay := loaded.DefaultDisplayProfile()
	if loadedDisplay.Sidebars[0].Side != "left" || !loadedDisplay.Sidebars[1].Enabled {
		t.Fatalf("persisted sidebars = %+v", loadedDisplay.Sidebars)
	}
	view, ok := config.FindDisplayComponent(*loadedDisplay, customID)
	if !ok || view.SidebarHeight != 310 {
		t.Fatalf("persisted custom placement = %+v, %v", view, ok)
	}
	customViews := loaded.DefaultCharacterProfile().CustomViews
	if len(customViews) != 1 || customViews[0].ID != customID {
		t.Fatalf("persisted custom views = %+v", customViews)
	}

	before := loadedDisplay.Sidebars[0].Side
	if err := a.SetViewLayout(layout, []config.CustomTabConfig{{Name: ""}}); err == nil {
		t.Fatal("SetViewLayout accepted an empty custom-view name")
	}
	if cfg.DefaultDisplayProfile().Sidebars[0].Side != before {
		t.Fatal("invalid atomic update mutated the live layout")
	}
}

func TestSetSelectedActionSetIDPersistsStableSelection(t *testing.T) {
	cfg := config.Defaults()
	cfg.DefaultCharacterProfile().ActionSets = []config.ActionSet{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewGuiApp(&Deps{Config: cfg, ConfigPath: path}, &captureEmitter{})

	if err := a.SetSelectedActionSetID("two"); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.DefaultCharacterProfile().SelectedActionSetID; got != "two" {
		t.Fatalf("selected action set = %q, want two", got)
	}
	if err := a.SetSelectedActionSetID("missing"); err == nil {
		t.Fatal("accepted missing action set ID")
	}
}
