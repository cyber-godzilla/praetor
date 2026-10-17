package gui

import (
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
)

const testCharacterProfileID = "2a764b7a-4145-44d9-9574-3bc2f0fe29cd"

func newCharacterProfileTestApp(t *testing.T) (*GuiApp, *captureEmitter) {
	t.Helper()
	cfg := config.Defaults()
	cfg.DefaultCharacterProfile().Variables = map[string]string{"weapon": "gladius"}
	cfg.DefaultCharacterProfile().CustomViews = []config.CustomTabConfig{{
		ID: "1fc95886-1541-4db4-8a49-d30391439c65", Name: "Think/OOC", Visible: true,
	}}
	cfg.DefaultCharacterProfile().ActionSets = []config.ActionSet{{ID: "combat", Name: "Combat"}}
	c, err := client.NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Engine.Close)
	emitter := &captureEmitter{}
	app := NewGuiApp(&Deps{Client: c, Config: cfg, ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}, emitter)
	return app, emitter
}

func TestSetCharacterProfilesAddsFromDefaultAndManagesIdentity(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	identities := []config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: " Marcus ", Account: " PlayerOne ", CharacterNumber: 1},
	}
	if err := a.SetCharacterProfiles(identities); err != nil {
		t.Fatalf("SetCharacterProfiles(add): %v", err)
	}
	profiles := a.cfg().CharacterProfiles.Profiles
	if len(profiles) != 2 {
		t.Fatalf("profiles = %+v", profiles)
	}
	added := profiles[1]
	if added.Name != "Marcus" || added.Account != "PlayerOne" || added.CharacterNumber != 1 {
		t.Fatalf("added identity = %+v", added)
	}
	if added.Variables["weapon"] != "gladius" || len(added.ActionSets) != 1 {
		t.Fatalf("new character did not inherit Default: %+v", added)
	}
	if len(added.CustomViews) != 1 || added.CustomViews[0].Name != "Think/OOC" ||
		added.CustomViews[0].ID == a.cfg().DefaultCharacterProfile().CustomViews[0].ID ||
		!config.ValidUUID(added.CustomViews[0].ID) {
		t.Fatalf("inherited custom view did not receive a unique component UUID: %+v", added.CustomViews)
	}

	identities[1].Name = "Marcellus"
	if err := a.SetCharacterProfiles(identities); err != nil {
		t.Fatalf("SetCharacterProfiles(rename): %v", err)
	}
	if got := a.cfg().CharacterProfiles.Profiles[1].Name; got != "Marcellus" {
		t.Fatalf("renamed profile = %q", got)
	}

	if err := a.SetCharacterProfiles(identities[:1]); err != nil {
		t.Fatalf("SetCharacterProfiles(delete): %v", err)
	}
	if len(a.cfg().CharacterProfiles.Profiles) != 1 {
		t.Fatalf("profile was not deleted: %+v", a.cfg().CharacterProfiles.Profiles)
	}
}

func TestSetCharacterProfilesRejectsInvalidIdentityChanges(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	defaultIdentity := config.CharacterProfileIdentity{ID: config.DefaultCharacterProfileID, Name: "Default"}
	for name, identities := range map[string][]config.CharacterProfileIdentity{
		"missing default": {{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1}},
		"empty name":      {defaultIdentity, {ID: testCharacterProfileID, Name: " ", Account: "one", CharacterNumber: 1}},
		"empty account":   {defaultIdentity, {ID: testCharacterProfileID, Name: "Marcus", Account: " ", CharacterNumber: 1}},
		"zero number":     {defaultIdentity, {ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 0}},
		"duplicate account number": {
			defaultIdentity,
			{ID: testCharacterProfileID, Name: "Marcus", Account: "One", CharacterNumber: 1},
			{ID: "f7137849-3447-4408-830c-207af85c747d", Name: "Lucia", Account: "one", CharacterNumber: 1},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := a.SetCharacterProfiles(identities); err == nil {
				t.Fatal("SetCharacterProfiles accepted invalid identities")
			}
		})
	}
}

func TestObserveCharacterSelectionSupportsOneAndTwoCommandForms(t *testing.T) {
	a, emitter := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "PlayerOne", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("playerone")

	a.observeCharacterSelection("Play")
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("profile switched before number: %q", got)
	}
	a.observeCharacterSelection("1")
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("two-command switch = %q", got)
	}

	a.activateCharacterProfile(config.DefaultCharacterProfileID)
	a.observeCharacterSelection("  PLAY 1 ")
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("one-command switch = %q", got)
	}
	if got := a.inputVariables.Load().(map[string]string)["weapon"]; got != "gladius" {
		t.Fatalf("live variables after switch = %q", got)
	}

	found := false
	for _, emitted := range emitter.snapshot() {
		batch, ok := emitted.data.([]WireEvent)
		if !ok {
			continue
		}
		for _, event := range batch {
			if event.Kind == KindCharacterProfile && event.CharacterProfile != nil && event.CharacterProfile.ID == testCharacterProfileID {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("character profile switch was not emitted")
	}
}

func TestObserveCharacterSelectionScopesNumbersByAccountAndCancelsPendingPlay(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
		{ID: "f7137849-3447-4408-830c-207af85c747d", Name: "Lucia", Account: "two", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("two")
	a.observeCharacterSelection("play 1")
	if got := a.ActiveCharacterProfileID(); got != "f7137849-3447-4408-830c-207af85c747d" {
		t.Fatalf("account-scoped switch = %q", got)
	}

	a.activateCharacterProfile(config.DefaultCharacterProfileID)
	a.observeCharacterSelection("play")
	a.observeCharacterSelection("look")
	a.observeCharacterSelection("1")
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("unrelated command did not cancel pending play: %q", got)
	}
}

func TestFailedSendDoesNotSwitchCharacterProfile(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("one")
	if err := a.SendInput("play 1"); err == nil {
		t.Fatal("SendInput succeeded without a game connection")
	}
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("failed send switched profile to %q", got)
	}
}

func TestCharacterScopedSetterTargetsActiveProfile(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.activateCharacterProfile(testCharacterProfileID)
	if err := a.SetInputVariables(map[string]string{"target": "raider"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.cfg().DefaultCharacterProfile().Variables["target"]; ok {
		t.Fatal("active character variable was written into Default")
	}
	var active config.CharacterProfileConfig
	for _, profile := range a.cfg().CharacterProfiles.Profiles {
		if profile.ID == testCharacterProfileID {
			active = profile
		}
	}
	if got := active.Variables["target"]; got != "raider" {
		t.Fatalf("active profile target = %q", got)
	}
}

func TestActiveCharacterCustomViewSurvivesGlobalLayoutValidation(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.activateCharacterProfile(testCharacterProfileID)
	customID := "f7137849-3447-4408-830c-207af85c747d"
	custom := []config.CustomTabConfig{{ID: customID, Name: "Marcus chat", Visible: true}}
	layout := cloneConfig(a.cfg()).DisplayProfiles.Profiles[0]
	layout.Main = append(layout.Main, config.DisplayComponentConfig{ComponentID: customID})
	if err := a.SetViewLayout(layout, custom); err != nil {
		t.Fatal(err)
	}
	if _, ok := config.FindDisplayComponent(*a.cfg().DefaultDisplayProfile(), customID); !ok {
		t.Fatal("active character custom component was removed from the validated display layout")
	}
	var active *config.CharacterProfileConfig
	for i := range a.cfg().CharacterProfiles.Profiles {
		if a.cfg().CharacterProfiles.Profiles[i].ID == testCharacterProfileID {
			active = &a.cfg().CharacterProfiles.Profiles[i]
		}
	}
	if active == nil || len(active.CustomViews) != 1 || active.CustomViews[0].ID != customID {
		t.Fatalf("active custom views = %+v", active)
	}
}
