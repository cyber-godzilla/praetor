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
	if len(added.Variables) != 1 || added.Variables["weapon"] != "gladius" || added.ActionSetsMode != config.CharacterSettingInherit || len(added.ActionSets) != 0 {
		t.Fatalf("new character did not copy Default variables and inherit collections: %+v", added)
	}
	if got := a.cfg().ResolvedVariables(added.ID)["weapon"]; got != "gladius" {
		t.Fatalf("copied weapon = %q", got)
	}
	if got := a.cfg().ResolvedActionSets(added.ID); len(got) != 1 || got[0].ID != "combat" {
		t.Fatalf("resolved inherited action sets = %+v", got)
	}
	if len(added.CustomViews) != 1 || added.CustomViews[0].Name != "Think/OOC" ||
		added.CustomViews[0].ID == a.cfg().DefaultCharacterProfile().CustomViews[0].ID ||
		!config.ValidUUID(added.CustomViews[0].ID) {
		t.Fatalf("inherited custom view did not receive a unique component UUID: %+v", added.CustomViews)
	}
	characterLayout := a.cfg().DisplayProfileForCharacter(testCharacterProfileID)
	if characterLayout == nil || characterLayout.CharacterProfileID != testCharacterProfileID {
		t.Fatalf("inherited character layout = %+v", characterLayout)
	}
	if _, ok := config.FindDisplayComponent(*characterLayout, added.CustomViews[0].ID); !ok {
		t.Fatalf("inherited custom view missing from character layout: %+v", characterLayout)
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
	if len(a.cfg().DisplayProfiles.Profiles) != 1 {
		t.Fatalf("deleted character's display profile was retained: %+v", a.cfg().DisplayProfiles.Profiles)
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
		"duplicate name": {
			defaultIdentity,
			{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
			{ID: "f7137849-3447-4408-830c-207af85c747d", Name: " marcus ", Account: "two", CharacterNumber: 1},
		},
		"reserved default name": {
			defaultIdentity,
			{ID: testCharacterProfileID, Name: "default", Account: "one", CharacterNumber: 1},
		},
		"reserved global name": {
			defaultIdentity,
			{ID: testCharacterProfileID, Name: "GLOBAL", Account: "one", CharacterNumber: 1},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := a.SetCharacterProfiles(identities); err == nil {
				t.Fatal("SetCharacterProfiles accepted invalid identities")
			}
		})
	}
}

func TestManualCharacterSelectionByNameAndGlobalAlias(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus Aurelius", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SelectCharacterProfile("marcus aurelius", false); err != nil {
		t.Fatal(err)
	}
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("selected profile = %q", got)
	}
	if err := a.SelectCharacterProfile("global", false); err != nil {
		t.Fatal(err)
	}
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("global alias selected profile = %q", got)
	}
}

func TestSwitchingToActiveCharacterProfileIsNoOp(t *testing.T) {
	a, emitter := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err != nil {
		t.Fatal(err)
	}

	before := len(emitter.snapshot())
	if err := a.SwitchCharacterProfile(testCharacterProfileID, true); err != nil {
		t.Fatal(err)
	}
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("same-profile selection changed active ID to %q", got)
	}
	if after := len(emitter.snapshot()); after != before {
		t.Fatalf("same-profile selection emitted %d additional events", after-before)
	}
}

func TestCharacterSlashCommandOpensMenuOrSelectsQuotedName(t *testing.T) {
	a, emitter := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus Aurelius", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SendInput(`/character "Marcus Aurelius"`); err != nil {
		t.Fatal(err)
	}
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("slash-selected profile = %q", got)
	}
	if err := a.SendInput("/character"); err != nil {
		t.Fatal(err)
	}
	for _, emitted := range emitter.snapshot() {
		if events, ok := emitted.data.([]WireEvent); ok {
			for _, event := range events {
				if event.Kind == KindOpenMenu && event.OpenMenu == "menu-character" {
					return
				}
			}
		}
	}
	t.Fatal("bare /character did not open the Character settings menu")
}

func TestCharacterScopedQuickCyclePriorityAndActionSetSetters(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	a.cfg().UI.QuickCycleModes = []string{"disable", "combat"}
	a.cfg().Commands.HighPriority = []string{"flee"}
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetQuickCycleModes([]string{}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetHighPriority([]string{"retreat"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetActionSets([]config.ActionSet{}); err != nil {
		t.Fatal(err)
	}
	profile := a.cfg().CharacterProfileByID(testCharacterProfileID)
	if profile.QuickCycleMode != config.CharacterSettingCustomize || profile.QuickCycleModes == nil || len(profile.QuickCycleModes) != 0 {
		t.Fatalf("quick cycle customization = %+v", profile)
	}
	if profile.PriorityCommandsMode != config.CharacterSettingCustomize || len(profile.HighPriority) != 1 || profile.HighPriority[0] != "retreat" {
		t.Fatalf("priority customization = %+v", profile)
	}
	if profile.ActionSetsMode != config.CharacterSettingCustomize || profile.ActionSets == nil || len(profile.ActionSets) != 0 {
		t.Fatalf("action set customization = %+v", profile)
	}
	if len(a.cfg().UI.QuickCycleModes) != 2 || a.cfg().Commands.HighPriority[0] != "flee" {
		t.Fatal("character setters modified Default values")
	}
}

func TestCharacterActionSetEditDoesNotPropagateToAnotherProfile(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	profile := a.cfg().CharacterProfileByID(testCharacterProfileID)
	profile.ActionSetsMode = config.CharacterSettingCustomize
	profile.ActionSets = a.cfg().ResolvedActionSets(testCharacterProfileID)
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.SetActionSets([]config.ActionSet{{ID: "combat", Name: "Renamed", Buttons: []config.ActionButton{{Label: "Hit", Command: "attack"}}}}); err != nil {
		t.Fatal(err)
	}
	if got := a.cfg().DefaultCharacterProfile().ActionSets[0].Name; got != "Combat" {
		t.Fatalf("character edit propagated to Default: %q", got)
	}
	if err := a.SetActionSets([]config.ActionSet{}); err != nil {
		t.Fatal(err)
	}
	if len(a.cfg().CharacterProfileByID(testCharacterProfileID).ActionSets) != 0 {
		t.Fatal("reference removal retained active membership")
	}
	if len(a.cfg().DefaultCharacterProfile().ActionSets) != 1 {
		t.Fatal("profile removal deleted another profile's Action Set")
	}
}

func TestSetCharacterProfilesRollsBackOnSaveFailure(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	a.deps.ConfigPath = filepath.Join(t.TempDir(), "missing", "config.yaml")
	err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	})
	if err == nil {
		t.Fatal("SetCharacterProfiles succeeded with an unwritable config path")
	}
	if got := len(a.cfg().CharacterProfiles.Profiles); got != 1 {
		t.Fatalf("failed save retained %d profiles", got)
	}
}

func TestCharacterSwitchRequiresExplicitStopForActiveLuaMode(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	modes := a.client().Engine.ModeNames()
	if len(modes) == 0 {
		t.Skip("test fixture has no Lua modes")
	}
	a.client().Engine.SetMode(modes[0], nil)
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err == nil {
		t.Fatal("switch succeeded without permission to stop the active Lua mode")
	}
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("blocked switch selected %q", got)
	}
	if err := a.SwitchCharacterProfile(testCharacterProfileID, true); err != nil {
		t.Fatal(err)
	}
	if got := a.client().Engine.CurrentMode(); got != "disable" && got != "" {
		t.Fatalf("mode after explicit stop = %q", got)
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

func TestObservedPlaySwitchCanRunInsideSendActivityTransaction(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("one")

	a.activityMu.Lock()
	a.observeCharacterSelectionLocked("play 1")
	a.activityMu.Unlock()

	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("observed play selected %q", got)
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

func TestObserveCharacterSelectionSleepReturnsToDefault(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.activateCharacterProfile(testCharacterProfileID)
	a.observeCharacterSelection("  SLEEP ")
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("sleep left profile active: %q", got)
	}
}

func TestCharacterProfileAutoSwitchCanBeDisabled(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("one")
	if err := a.SetCharacterProfileAutoSwitch(false); err != nil {
		t.Fatal(err)
	}
	a.observeCharacterSelection("play 1")
	if got := a.ActiveCharacterProfileID(); got != config.DefaultCharacterProfileID {
		t.Fatalf("disabled play switching selected %q", got)
	}
	a.activateCharacterProfile(testCharacterProfileID)
	a.observeCharacterSelection("sleep")
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("disabled sleep switching selected %q", got)
	}
	loaded, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CharacterProfiles.AutoSwitch {
		t.Fatal("disabled automatic switching was not persisted")
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

func TestFailedSleepSendDoesNotReturnToDefault(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.setConnectedAccount("one")
	a.activateCharacterProfile(testCharacterProfileID)
	if err := a.SendInput("sleep"); err == nil {
		t.Fatal("SendInput succeeded without a game connection")
	}
	if got := a.ActiveCharacterProfileID(); got != testCharacterProfileID {
		t.Fatalf("failed sleep send selected %q", got)
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

func TestCharacterScopedVariableDeletionPersistsAcrossReload(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	a.cfg().DefaultCharacterProfile().Variables = map[string]string{
		"target": "rat",
		"stance": "guard",
	}
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	a.activateCharacterProfile(testCharacterProfileID)

	// SetInputVariables receives the complete list still visible in the editor.
	// Omitting target means it was explicitly deleted for this character.
	if err := a.SetInputVariables(map[string]string{"stance": "guard"}); err != nil {
		t.Fatal(err)
	}
	profile := a.cfg().CharacterProfileByID(testCharacterProfileID)
	if len(profile.Variables) != 1 || profile.Variables["stance"] != "guard" {
		t.Fatalf("character variables = %#v, want only stance", profile.Variables)
	}
	if _, exists := a.cfg().ResolvedVariables(profile.ID)["target"]; exists {
		t.Fatal("deleted target returned to the character list")
	}
	if got := a.cfg().ResolvedVariables(profile.ID)["stance"]; got != "guard" {
		t.Fatalf("remaining stance = %q, want guard", got)
	}
	if _, exists := a.inputVariables.Load().(map[string]string)["target"]; exists {
		t.Fatal("deleted target remained in the live variable snapshot")
	}
	reloaded, err := config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	reloadedProfile := reloaded.CharacterProfileByID(testCharacterProfileID)
	if _, exists := reloadedProfile.Variables["target"]; exists {
		t.Fatalf("deleted target returned after application reload: %#v", reloadedProfile.Variables)
	}

	if err := a.SetInputVariables(map[string]string{}); err != nil {
		t.Fatal(err)
	}
	reloaded, err = config.Load(a.deps.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if variables := reloaded.CharacterProfileByID(testCharacterProfileID).Variables; len(variables) != 0 {
		t.Fatalf("empty character variable list returned after reload as %#v", variables)
	}
}

func TestSetVariableCommandUpdatesTheIndependentCharacterList(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	a.cfg().DefaultCharacterProfile().Variables["stance"] = "guard"
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.SendInput(`/set-variable weapon spear`); err != nil {
		t.Fatal(err)
	}
	profile := a.cfg().CharacterProfileByID(testCharacterProfileID)
	if len(profile.Variables) != 2 || profile.Variables["weapon"] != "spear" {
		t.Fatalf("character variables = %#v", profile.Variables)
	}
	if got := a.cfg().ResolvedVariables(profile.ID)["stance"]; got != "guard" {
		t.Fatalf("copied stance = %q", got)
	}
}

func TestLuaVariableWriteTargetsActiveCharacterOverride(t *testing.T) {
	a, _ := newCharacterProfileTestApp(t)
	if err := a.SetCharacterProfiles([]config.CharacterProfileIdentity{
		{ID: config.DefaultCharacterProfileID, Name: "Default"},
		{ID: testCharacterProfileID, Name: "Marcus", Account: "one", CharacterNumber: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.SwitchCharacterProfile(testCharacterProfileID, false); err != nil {
		t.Fatal(err)
	}
	if err := a.client().Engine.OnVariableSet("crafting_material", "gold"); err != nil {
		t.Fatal(err)
	}

	if _, ok := a.cfg().DefaultCharacterProfile().Variables["crafting_material"]; ok {
		t.Fatal("Lua variable write leaked into Default")
	}
	profile := a.cfg().CharacterProfileByID(testCharacterProfileID)
	if got := profile.Variables["crafting_material"]; got != "gold" {
		t.Fatalf("character crafting_material = %q, want gold", got)
	}
	if got, ok := a.client().Engine.OnVariableGet("crafting_material"); !ok || got != "gold" {
		t.Fatalf("live Lua variable = %q, %v", got, ok)
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
	layout := *cloneConfig(a.cfg()).DisplayProfileForCharacter(testCharacterProfileID)
	layout.Main = append(layout.Main, config.DisplayComponentConfig{ComponentID: customID})
	if err := a.SetViewLayout(layout, custom); err != nil {
		t.Fatal(err)
	}
	if _, ok := config.FindDisplayComponent(*a.cfg().DisplayProfileForCharacter(testCharacterProfileID), customID); !ok {
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
