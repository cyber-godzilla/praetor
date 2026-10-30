package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultsIncludeGlobalUUIDDisplayProfile(t *testing.T) {
	cfg := Defaults()
	profile := cfg.DefaultDisplayProfile()
	if profile == nil {
		t.Fatal("DefaultDisplayProfile() = nil")
	}
	if !ValidUUID(profile.ID) || cfg.DisplayProfiles.DefaultID != profile.ID {
		t.Fatalf("default display identity = %q / %q", cfg.DisplayProfiles.DefaultID, profile.ID)
	}
	if profile.Name != "Default" || !profile.SidebarsVisible {
		t.Fatalf("default display profile = %+v", profile)
	}
	if len(profile.Sidebars) != 3 {
		t.Fatalf("sidebars = %d, want 3 initial sidebars", len(profile.Sidebars))
	}
	seen := map[string]bool{}
	for i, sidebar := range profile.Sidebars {
		if !ValidUUID(sidebar.ID) || seen[sidebar.ID] {
			t.Fatalf("sidebar[%d] has invalid/duplicate UUID %q", i, sidebar.ID)
		}
		seen[sidebar.ID] = true
	}
	if got := componentIDs(profile.Main); len(got) < 1 || got[0] != ComponentIDAll {
		t.Fatalf("main components = %v, want All first", got)
	}
	wantSidebar := []string{ComponentIDMap, ComponentIDCompass, ComponentIDVitals, ComponentIDActions, ComponentIDModes, ComponentIDVariables}
	if got := componentIDs(profile.Sidebars[0].Components); !equalStrings(got, wantSidebar) {
		t.Fatalf("sidebar components = %v, want %v", got, wantSidebar)
	}
	for _, id := range append(componentIDs(profile.Main), wantSidebar...) {
		if !ValidUUID(id) {
			t.Fatalf("component ID %q is not a UUID", id)
		}
	}
}

func TestLoadMigratesLegacyGUILayoutIntoDefaultDisplayProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	data := []byte(`server:
  host: example.test
  port: 443
  protocol: wss
  login_url: https://example.test/login
ui:
  display_mode: topbar
  gui_sidebar_width: 344
  gui_minimap_height: 233
  custom_tabs:
    - id: legacy-chat
      name: Chat
      visible: true
      rules: []
  gui_layout:
    sidebars_visible: true
    sidebars:
      - {id: 1, enabled: true, side: left, width: 344}
      - {id: 2, enabled: true, side: right, width: 280}
    views:
      - {id: map, location: sidebar-1, priority: 900, sidebar_height: 233}
      - {id: legacy-chat, location: sidebar-2, priority: 100, sidebar_height: 310}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.DisplayMode != "topbar" {
		t.Fatalf("TUI DisplayMode = %q, want topbar", cfg.UI.DisplayMode)
	}
	profile := cfg.DefaultDisplayProfile()
	if len(profile.Sidebars) != 2 || profile.Sidebars[0].Side != "left" || profile.Sidebars[0].Width != 344 {
		t.Fatalf("migrated sidebars = %+v", profile.Sidebars)
	}
	mapPlacement, ok := FindDisplayComponent(*profile, ComponentIDMap)
	if !ok || mapPlacement.SidebarHeight != 233 {
		t.Fatalf("migrated map = %+v, %v", mapPlacement, ok)
	}
	custom := cfg.DefaultCharacterProfile().CustomViews
	if len(custom) != 1 || !ValidUUID(custom[0].ID) {
		t.Fatalf("migrated custom views = %+v", custom)
	}
	if got := componentIDs(profile.Sidebars[1].Components); len(got) != 1 || got[0] != custom[0].ID {
		t.Fatalf("migrated Sidebar 2 components = %v, want custom component %q", got, custom[0].ID)
	}
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "display_profiles:") || strings.Contains(string(saved), "gui_layout:") {
		t.Fatalf("canonical display profile was not saved cleanly:\n%s", saved)
	}
}

func TestValidateRepairsDisplayProfileWithoutThreeSidebarLimit(t *testing.T) {
	cfg := Defaults()
	customID := "1fc95886-1541-4db4-8a49-d30391439c65"
	cfg.DefaultCharacterProfile().CustomViews = []CustomTabConfig{{ID: customID, Name: "Chat", Visible: true}}
	profile := cfg.DefaultDisplayProfile()
	profile.Main = []DisplayComponentConfig{
		{ComponentID: ComponentIDAll},
		{ComponentID: ComponentIDMap},
		{ComponentID: ComponentIDMap},
		{ComponentID: "unknown"},
	}
	profile.Sidebars = []DisplaySidebarConfig{
		{ID: "bad", Name: "", Side: "sideways", Width: 999, Components: []DisplayComponentConfig{{ComponentID: customID, SidebarHeight: 1}}},
		{ID: "bad", Name: "Second", Side: "right", Width: 10},
		{ID: newUUID(), Name: "Third", Side: "left", Width: 260},
		{ID: newUUID(), Name: "Fourth", Side: "right", Width: 260},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(profile.Sidebars) != 4 {
		t.Fatalf("sidebars = %d, want all four preserved", len(profile.Sidebars))
	}
	seen := map[string]bool{}
	for i, sidebar := range profile.Sidebars {
		if !ValidUUID(sidebar.ID) || seen[sidebar.ID] {
			t.Fatalf("sidebar[%d] has invalid/duplicate ID %q", i, sidebar.ID)
		}
		seen[sidebar.ID] = true
	}
	if profile.Sidebars[0].Side != "right" || profile.Sidebars[0].Width != 600 || profile.Sidebars[0].Name != "Sidebar 1" {
		t.Fatalf("sidebar repair = %+v", profile.Sidebars[0])
	}
	if got := componentIDs(profile.Main); !equalStrings(got, []string{ComponentIDAll, ComponentIDMap}) {
		t.Fatalf("normalized main = %v", got)
	}
	wantBench := []string{ComponentIDMetrics, ComponentIDDebug, ComponentIDCompass, ComponentIDVitals, ComponentIDActions, ComponentIDModes, ComponentIDVariables}
	if got := componentIDs(profile.Bench); !equalStrings(got, wantBench) {
		t.Fatalf("normalized bench = %v, want %v", got, wantBench)
	}
	customPlacement, ok := FindDisplayComponent(*profile, customID)
	if !ok || customPlacement.SidebarHeight != 300 {
		t.Fatalf("custom placement = %+v, %v", customPlacement, ok)
	}
}

func TestValidatePreservesBenchOrderAndSidebarState(t *testing.T) {
	cfg := Defaults()
	profile := cfg.DefaultDisplayProfile()
	profile.Bench = []DisplayComponentConfig{
		{ComponentID: ComponentIDMap, SidebarHeight: 233, Collapsed: true},
		{ComponentID: ComponentIDVitals, SidebarHeight: 144},
	}
	profile.Sidebars[0].Components = []DisplayComponentConfig{
		{ComponentID: ComponentIDCompass},
		{ComponentID: ComponentIDActions},
		{ComponentID: ComponentIDModes},
		{ComponentID: ComponentIDVariables},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	got := componentIDs(profile.Bench)
	if len(got) < 2 || !equalStrings(got[:2], []string{ComponentIDMap, ComponentIDVitals}) {
		t.Fatalf("bench order = %v", got)
	}
	if profile.Bench[0].SidebarHeight != 233 || !profile.Bench[0].Collapsed {
		t.Fatalf("bench state = %+v", profile.Bench[0])
	}
}

func TestValidateCreatesCharacterDisplayProfileAndRemapsCustomViews(t *testing.T) {
	cfg := Defaults()
	defaultViewID := "1fc95886-1541-4db4-8a49-d30391439c65"
	characterViewID := "f7137849-3447-4408-830c-207af85c747d"
	characterID := "2a764b7a-4145-44d9-9574-3bc2f0fe29cd"
	cfg.DefaultCharacterProfile().CustomViews = []CustomTabConfig{{ID: defaultViewID, Name: "Think/OOC", Visible: true}}
	cfg.DefaultDisplayProfile().Main = append(cfg.DefaultDisplayProfile().Main, DisplayComponentConfig{ComponentID: defaultViewID})
	cfg.CharacterProfiles.Profiles = append(cfg.CharacterProfiles.Profiles, CharacterProfileConfig{
		ID: characterID, Name: "Marcus", Account: "one", CharacterNumber: 1,
		Variables: map[string]string{}, CustomViews: []CustomTabConfig{{ID: characterViewID, Name: "Think/OOC", Visible: true}}, ActionSets: []ActionSet{},
	})

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	characterLayout := cfg.DisplayProfileForCharacter(characterID)
	if characterLayout == nil || characterLayout.ID == cfg.DisplayProfiles.DefaultID {
		t.Fatalf("character layout = %+v", characterLayout)
	}
	if characterLayout.CharacterProfileID != characterID {
		t.Fatalf("layout character = %q, want %q", characterLayout.CharacterProfileID, characterID)
	}
	if _, ok := FindDisplayComponent(*characterLayout, characterViewID); !ok {
		t.Fatalf("character custom view missing from layout: %+v", characterLayout)
	}
	if _, ok := FindDisplayComponent(*characterLayout, defaultViewID); ok {
		t.Fatalf("default custom view leaked into character layout: %+v", characterLayout)
	}
	if _, ok := FindDisplayComponent(*cfg.DefaultDisplayProfile(), characterViewID); ok {
		t.Fatalf("character custom view leaked into global Default layout")
	}
	if characterLayout.Sidebars[0].ID == cfg.DefaultDisplayProfile().Sidebars[0].ID {
		t.Fatal("inherited character sidebar reused the Default sidebar UUID")
	}
	seenSidebars := map[string]bool{}
	for _, display := range cfg.DisplayProfiles.Profiles {
		for _, sidebar := range display.Sidebars {
			if seenSidebars[sidebar.ID] {
				t.Fatalf("sidebar UUID %q collides across display profiles", sidebar.ID)
			}
			seenSidebars[sidebar.ID] = true
		}
	}
}

func componentIDs(components []DisplayComponentConfig) []string {
	ids := make([]string, len(components))
	for i, component := range components {
		ids[i] = component.ComponentID
	}
	return ids
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
