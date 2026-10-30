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
	layout.Main = []config.DisplayComponentConfig{layout.Main[0], layout.Main[2]}
	layout.Bench = []config.DisplayComponentConfig{{
		ComponentID: config.ComponentIDMetrics, SidebarHeight: 275, Collapsed: true,
	}}
	layout.Sidebars = append([]config.DisplaySidebarConfig(nil), layout.Sidebars...)
	for i := range layout.Sidebars {
		layout.Sidebars[i].Components = append([]config.DisplayComponentConfig(nil), layout.Sidebars[i].Components...)
	}
	layout.Sidebars[0].Side = "left"
	layout.Sidebars[0].Collapsed = true
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
	if loadedDisplay.Sidebars[0].Side != "left" || !loadedDisplay.Sidebars[0].Collapsed || !loadedDisplay.Sidebars[1].Enabled {
		t.Fatalf("persisted sidebars = %+v", loadedDisplay.Sidebars)
	}
	if len(loadedDisplay.Bench) == 0 || loadedDisplay.Bench[0].ComponentID != config.ComponentIDMetrics ||
		loadedDisplay.Bench[0].SidebarHeight != 275 || !loadedDisplay.Bench[0].Collapsed {
		t.Fatalf("persisted bench = %+v", loadedDisplay.Bench)
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

func TestSetViewLayoutUpdatesTheActiveCharactersDisplayProfile(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.activateCharacterProfile(testCharacterProfileID)
	characterLayout := a.cfg().DisplayProfileForCharacter(testCharacterProfileID)
	defaultSide := a.cfg().DefaultDisplayProfile().Sidebars[0].Side
	layout := *characterLayout
	layout.Sidebars = append([]config.DisplaySidebarConfig(nil), characterLayout.Sidebars...)
	layout.Sidebars[0].Side = "left"
	custom := append([]config.CustomTabConfig(nil), a.activeCharacterProfileLocked().CustomViews...)
	if err := a.SetViewLayout(layout, custom); err != nil {
		t.Fatal(err)
	}
	if got := a.cfg().DisplayProfileForCharacter(testCharacterProfileID).Sidebars[0].Side; got != "left" {
		t.Fatalf("active character sidebar side = %q", got)
	}
	if got := a.cfg().DefaultDisplayProfile().Sidebars[0].Side; got != defaultSide {
		t.Fatalf("global Default layout changed to %q", got)
	}
	loaded, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.DisplayProfileForCharacter(testCharacterProfileID).Sidebars[0].Side; got != "left" {
		t.Fatalf("reloaded character sidebar side = %q", got)
	}
}

func TestSetProfileViewLayoutCanEditANonActiveCharacterFromTheGlobalLibrary(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	layout := *a.cfg().DisplayProfileForCharacter(testCharacterProfileID)
	custom := append([]config.CustomTabConfig(nil), a.cfg().CharacterProfileByID(testCharacterProfileID).CustomViews...)
	custom[0].Name = "Marcus private view"
	if err := a.SetProfileViewLayout(testCharacterProfileID, layout, custom); err != nil {
		t.Fatal(err)
	}
	if got := a.cfg().CharacterProfileByID(testCharacterProfileID).CustomViews[0].Name; got != "Marcus private view" {
		t.Fatalf("non-active character view name = %q", got)
	}
	if got := a.cfg().DefaultCharacterProfile().CustomViews[0].Name; got != "Think/OOC" {
		t.Fatalf("global library edit changed Default view to %q", got)
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
