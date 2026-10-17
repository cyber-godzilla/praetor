package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultsContainGlobalDefaultCharacterProfile(t *testing.T) {
	cfg := Defaults()
	profile := cfg.DefaultCharacterProfile()
	if profile == nil {
		t.Fatal("DefaultCharacterProfile() = nil")
	}
	if cfg.CharacterProfiles.DefaultID != profile.ID {
		t.Fatalf("default ID = %q, profile ID = %q", cfg.CharacterProfiles.DefaultID, profile.ID)
	}
	if !ValidUUID(profile.ID) {
		t.Fatalf("default profile ID %q is not a UUID", profile.ID)
	}
	if profile.Name != "Default" {
		t.Fatalf("default profile name = %q", profile.Name)
	}
	if profile.Variables == nil || profile.CustomViews == nil || profile.ActionSets == nil {
		t.Fatalf("default profile collections must be initialized: %+v", profile)
	}
	if len(cfg.CharacterProfiles.Profiles) != 1 {
		t.Fatalf("fresh config profiles = %d, want only Default", len(cfg.CharacterProfiles.Profiles))
	}
}

func TestLoadMigratesLegacyCharacterScopedSettingsIntoDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := `
server:
  host: game.example.test
  port: 8080
  protocol: ws
  login_url: https://example.test/login
commands:
  variables:
    target: scarred bandit
ui:
  custom_tabs:
    - name: Think/OOC
      visible: true
      rules:
        - pattern: thinks aloud
          include: true
          active: true
  action_sets:
    - id: combat
      name: Combat
      buttons:
        - label: Attack
          command: attack ${target}
  selected_action_set_id: combat
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	profile := cfg.DefaultCharacterProfile()
	if got := profile.Variables["target"]; got != "scarred bandit" {
		t.Fatalf("migrated variable = %q", got)
	}
	if len(profile.CustomViews) != 1 || profile.CustomViews[0].Name != "Think/OOC" || profile.CustomViews[0].ID == "" {
		t.Fatalf("migrated custom views = %+v", profile.CustomViews)
	}
	if len(profile.ActionSets) != 1 || profile.ActionSets[0].ID != "combat" {
		t.Fatalf("migrated action sets = %+v", profile.ActionSets)
	}
	if profile.SelectedActionSetID != "combat" {
		t.Fatalf("selected action set = %q", profile.SelectedActionSetID)
	}

	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["character_profiles"]; !ok {
		t.Fatalf("saved config lacks character_profiles:\n%s", data)
	}
	commands := raw["commands"].(map[string]any)
	if _, ok := commands["variables"]; ok {
		t.Fatalf("legacy commands.variables was written after migration:\n%s", data)
	}
	ui := raw["ui"].(map[string]any)
	for _, key := range []string{"custom_tabs", "action_sets", "selected_action_set_id"} {
		if _, ok := ui[key]; ok {
			t.Fatalf("legacy ui.%s was written after migration:\n%s", key, data)
		}
	}
	if strings.Count(string(data), "scarred bandit") != 1 {
		t.Fatalf("profile data was duplicated:\n%s", data)
	}
}

func TestValidateRepairsDefaultCharacterProfileIdentity(t *testing.T) {
	cfg := Defaults()
	cfg.CharacterProfiles.DefaultID = "not-a-uuid"
	cfg.CharacterProfiles.Profiles[0].ID = "also-not-a-uuid"
	cfg.CharacterProfiles.Profiles[0].Name = ""
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	profile := cfg.DefaultCharacterProfile()
	if profile == nil || !ValidUUID(profile.ID) || profile.Name != "Default" {
		t.Fatalf("repaired default profile = %+v", profile)
	}
	if cfg.CharacterProfiles.DefaultID != profile.ID {
		t.Fatalf("repaired default ID = %q, profile ID = %q", cfg.CharacterProfiles.DefaultID, profile.ID)
	}
}

func TestCharacterProfileIdentityRoundTrips(t *testing.T) {
	cfg := Defaults()
	cfg.CharacterProfiles.Profiles = append(cfg.CharacterProfiles.Profiles, CharacterProfileConfig{
		ID:              "2a764b7a-4145-44d9-9574-3bc2f0fe29cd",
		Name:            "Marcus",
		Account:         " PlayerOne ",
		CharacterNumber: 3,
	})
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	profile := cfg.CharacterProfiles.Profiles[1]
	if profile.Account != "PlayerOne" || profile.CharacterNumber != 3 {
		t.Fatalf("normalized character identity = account %q, number %d", profile.Account, profile.CharacterNumber)
	}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	profile = loaded.CharacterProfiles.Profiles[1]
	if profile.Account != "PlayerOne" || profile.CharacterNumber != 3 {
		t.Fatalf("loaded character identity = account %q, number %d", profile.Account, profile.CharacterNumber)
	}
}

func TestDefaultCharacterProfileHasNoGameCharacterIdentity(t *testing.T) {
	cfg := Defaults()
	profile := cfg.DefaultCharacterProfile()
	profile.Account = "should-be-cleared"
	profile.CharacterNumber = 4
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if profile.Account != "" || profile.CharacterNumber != 0 {
		t.Fatalf("default profile retained game identity: %+v", profile)
	}
}

func TestValidateMakesCustomComponentIDsUniqueAcrossCharacterProfiles(t *testing.T) {
	cfg := Defaults()
	sharedID := "1fc95886-1541-4db4-8a49-d30391439c65"
	cfg.DefaultCharacterProfile().CustomViews = []CustomTabConfig{{ID: sharedID, Name: "Default chat"}}
	cfg.CharacterProfiles.Profiles = append(cfg.CharacterProfiles.Profiles, CharacterProfileConfig{
		ID: "2a764b7a-4145-44d9-9574-3bc2f0fe29cd", Name: "Marcus", Account: "one", CharacterNumber: 1,
		CustomViews: []CustomTabConfig{{ID: sharedID, Name: "Marcus chat"}},
	})
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	got := cfg.CharacterProfiles.Profiles[1].CustomViews[0].ID
	if got == sharedID || !ValidUUID(got) {
		t.Fatalf("duplicate custom component ID repaired to %q", got)
	}
}
