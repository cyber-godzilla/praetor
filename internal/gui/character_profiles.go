package gui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/config"
)

// ActiveCharacterProfileID returns the session-scoped profile selection.
func (a *GuiApp) ActiveCharacterProfileID() string {
	a.characterMu.Lock()
	defer a.characterMu.Unlock()
	if a.activeProfileID == "" {
		return config.DefaultCharacterProfileID
	}
	return a.activeProfileID
}

func (a *GuiApp) setConnectedAccount(account string) {
	a.characterMu.Lock()
	a.connectedAccount = strings.TrimSpace(account)
	a.playSelectArmed = false
	a.characterMu.Unlock()
	a.configMu.RLock()
	defaultID := a.cfg().CharacterProfiles.DefaultID
	a.configMu.RUnlock()
	if a.ActiveCharacterProfileID() != defaultID {
		a.activateCharacterProfile(defaultID)
	}
}

func (a *GuiApp) activeCharacterPayload() *CharacterProfilePayload {
	id := a.ActiveCharacterProfileID()
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	for _, profile := range a.cfg().CharacterProfiles.Profiles {
		if profile.ID == id {
			return &CharacterProfilePayload{ID: profile.ID, Name: profile.Name}
		}
	}
	return nil
}

// SetCharacterProfiles applies identity-only edits from the profile manager.
// Existing content remains attached to its UUID; a new UUID inherits the
// global Default profile's character-scoped settings.
func (a *GuiApp) SetCharacterProfiles(identities []config.CharacterProfileIdentity) error {
	if len(identities) == 0 {
		return fmt.Errorf("the global Default profile is required")
	}
	a.configMu.RLock()
	defaultID := a.cfg().CharacterProfiles.DefaultID
	a.configMu.RUnlock()
	seenIDs := make(map[string]bool, len(identities))
	seenCharacters := make(map[string]bool, len(identities))
	seenNames := make(map[string]bool, len(identities))
	foundDefault := false
	for _, identity := range identities {
		idKey := strings.ToLower(strings.TrimSpace(identity.ID))
		if !config.ValidUUID(identity.ID) || seenIDs[idKey] {
			return fmt.Errorf("character profile IDs must be unique UUIDs")
		}
		seenIDs[idKey] = true
		if identity.ID == defaultID {
			foundDefault = true
			continue
		}
		name := strings.TrimSpace(identity.Name)
		account := strings.TrimSpace(identity.Account)
		if name == "" {
			return fmt.Errorf("character profile names must not be empty")
		}
		nameKey := strings.ToLower(name)
		if nameKey == "default" || nameKey == "global" {
			return fmt.Errorf("%q is reserved for the global Default profile", name)
		}
		if seenNames[nameKey] {
			return fmt.Errorf("character profile names must be unique")
		}
		seenNames[nameKey] = true
		if account == "" {
			return fmt.Errorf("character profiles require an account")
		}
		if identity.CharacterNumber <= 0 {
			return fmt.Errorf("character numbers must be positive")
		}
		characterKey := strings.ToLower(account) + "\x00" + strconv.Itoa(identity.CharacterNumber)
		if seenCharacters[characterKey] {
			return fmt.Errorf("account %q already has character number %d", account, identity.CharacterNumber)
		}
		seenCharacters[characterKey] = true
	}
	if !foundDefault {
		return fmt.Errorf("the global Default profile cannot be deleted")
	}

	activeID := a.ActiveCharacterProfileID()
	if !seenIDs[strings.ToLower(activeID)] {
		if err := a.SwitchCharacterProfile(defaultID, false); err != nil {
			return fmt.Errorf("switch away from the profile before deleting it: %w", err)
		}
		activeID = defaultID
	}
	a.configMu.Lock()
	previous := cloneConfig(a.cfg())
	existing := make(map[string]config.CharacterProfileConfig, len(a.cfg().CharacterProfiles.Profiles))
	for _, profile := range a.cfg().CharacterProfiles.Profiles {
		existing[profile.ID] = profile
	}
	removedIDs := make([]string, 0)
	for id := range existing {
		if id != defaultID && !seenIDs[strings.ToLower(id)] {
			removedIDs = append(removedIDs, id)
		}
	}
	defaultProfile := cloneCharacterProfile(*a.cfg().DefaultCharacterProfile())
	replacement := make([]config.CharacterProfileConfig, 0, len(identities))
	for _, identity := range identities {
		profile, ok := existing[identity.ID]
		if !ok {
			profile = cloneCharacterProfile(defaultProfile)
			profile.ID = identity.ID
			profile.VariablesMode = config.CharacterVariablesIndependent
			profile.ActionSetsMode = config.CharacterSettingInherit
			profile.ActionSets = []config.ActionSet{}
			profile.QuickCycleMode = config.CharacterSettingInherit
			profile.QuickCycleModes = nil
			profile.PriorityCommandsMode = config.CharacterSettingInherit
			profile.HighPriority = nil
			for viewIndex := range profile.CustomViews {
				profile.CustomViews[viewIndex].ID = config.NewUUID()
			}
		}
		if identity.ID == defaultID {
			profile.Account = ""
			profile.CharacterNumber = 0
		} else {
			profile.Name = strings.TrimSpace(identity.Name)
			profile.Account = strings.TrimSpace(identity.Account)
			profile.CharacterNumber = identity.CharacterNumber
		}
		replacement = append(replacement, profile)
	}
	a.cfg().CharacterProfiles.Profiles = replacement
	if err := a.cfg().Validate(); err != nil {
		*a.cfg() = *previous
		a.configMu.Unlock()
		return err
	}
	if err := a.save(); err != nil {
		a.configMu.Unlock()
		return err
	}
	a.configMu.Unlock()

	for _, id := range removedIDs {
		if err := a.client().Engine.DeletePersistenceNamespace("character:" + id); err != nil {
			return fmt.Errorf("delete Lua data for removed character: %w", err)
		}
	}
	return nil
}

func cloneCharacterProfile(source config.CharacterProfileConfig) config.CharacterProfileConfig {
	cloned := source
	cloned.Variables = cloneStringMap(source.Variables)
	cloned.CustomViews = append([]config.CustomTabConfig(nil), source.CustomViews...)
	cloned.Highlights = append([]config.HighlightConfig(nil), source.Highlights...)
	for i := range cloned.CustomViews {
		cloned.CustomViews[i].Rules = append([]config.TabRuleConfig(nil), source.CustomViews[i].Rules...)
	}
	cloned.ActionSets = append([]config.ActionSet(nil), source.ActionSets...)
	for i := range cloned.ActionSets {
		cloned.ActionSets[i].Buttons = append([]config.ActionButton(nil), source.ActionSets[i].Buttons...)
	}
	cloned.QuickCycleModes = append([]string(nil), source.QuickCycleModes...)
	cloned.HighPriority = append([]string(nil), source.HighPriority...)
	return cloned
}

// SetCharacterProfileAutoSwitch controls whether successful play and sleep
// commands change the active character profile. Disabling it also cancels a
// pending two-command play selection.
func (a *GuiApp) SetCharacterProfileAutoSwitch(enabled bool) error {
	if err := a.withConfig(func() { a.cfg().CharacterProfiles.AutoSwitch = enabled }); err != nil {
		return err
	}
	if !enabled {
		a.characterMu.Lock()
		a.playSelectArmed = false
		a.characterMu.Unlock()
	}
	return nil
}

func (a *GuiApp) observeCharacterSelection(input string) {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	a.observeCharacterSelectionLocked(input)
}

// observeCharacterSelectionLocked runs after a command has been accepted while
// SendInput still owns activityMu. It must use the locked switch path to avoid
// recursively acquiring that non-reentrant mutex.
func (a *GuiApp) observeCharacterSelectionLocked(input string) {
	a.configMu.RLock()
	autoSwitch := a.cfg().CharacterProfiles.AutoSwitch
	defaultID := a.cfg().CharacterProfiles.DefaultID
	a.configMu.RUnlock()
	if !autoSwitch {
		a.characterMu.Lock()
		a.playSelectArmed = false
		a.characterMu.Unlock()
		return
	}

	command := strings.TrimSpace(input)
	parts := strings.Fields(command)

	a.characterMu.Lock()
	account := a.connectedAccount
	armed := a.playSelectArmed
	a.playSelectArmed = false
	if len(parts) == 1 && strings.EqualFold(parts[0], "play") {
		a.playSelectArmed = true
		a.characterMu.Unlock()
		return
	}
	a.characterMu.Unlock()
	if len(parts) == 1 && strings.EqualFold(parts[0], "sleep") {
		if a.ActiveCharacterProfileID() != defaultID {
			a.activateCharacterProfileLocked(defaultID)
		}
		return
	}

	number := 0
	var err error
	if len(parts) == 2 && strings.EqualFold(parts[0], "play") {
		number, err = strconv.Atoi(parts[1])
	} else if armed && len(parts) == 1 {
		number, err = strconv.Atoi(parts[0])
	} else {
		return
	}
	if err != nil || number <= 0 || account == "" {
		return
	}

	a.configMu.RLock()
	profileID := ""
	for _, profile := range a.cfg().CharacterProfiles.Profiles {
		if strings.EqualFold(profile.Account, account) && profile.CharacterNumber == number {
			profileID = profile.ID
			break
		}
	}
	a.configMu.RUnlock()
	if profileID == "" {
		a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{
			Title:   "Character profile not found",
			Message: fmt.Sprintf("No profile is assigned to %s character %d.", account, number),
		}}})
		return
	}
	a.activateCharacterProfileLocked(profileID)
}

// SelectCharacterProfile resolves a user-facing profile name. "global" and
// "default" both select the session's Default profile.
func (a *GuiApp) SelectCharacterProfile(name string, stopAutomation bool) error {
	name = strings.TrimSpace(name)
	a.configMu.RLock()
	profileID := ""
	if strings.EqualFold(name, "global") || strings.EqualFold(name, "default") {
		profileID = a.cfg().CharacterProfiles.DefaultID
	} else {
		for _, profile := range a.cfg().CharacterProfiles.Profiles {
			if strings.EqualFold(profile.Name, name) {
				profileID = profile.ID
				break
			}
		}
	}
	a.configMu.RUnlock()
	if profileID == "" {
		return fmt.Errorf("unknown character profile %q", name)
	}
	return a.SwitchCharacterProfile(profileID, stopAutomation)
}

// SwitchCharacterProfile applies every character-scoped runtime value as one
// serialized operation. Active performances always block a switch. A visual
// caller may explicitly stop a Lua mode before switching.
func (a *GuiApp) SwitchCharacterProfile(profileID string, stopAutomation bool) error {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	return a.switchCharacterProfileLocked(profileID, stopAutomation)
}

// switchCharacterProfileLocked requires activityMu. Keeping the switch body in
// one place lets both external picker calls and successful SendInput observers
// preserve the same automation preflight without recursively locking.
func (a *GuiApp) switchCharacterProfileLocked(profileID string, stopAutomation bool) error {
	// Selecting the already-active profile is not a transition. Return before
	// automation preflight, persistence reloads, runtime updates, or frontend
	// events so the caller cannot reset otherwise-local UI state such as tabs.
	if a.ActiveCharacterProfileID() == profileID {
		return nil
	}
	a.waitForStoppedPlay()
	if a.PlayActive() {
		return fmt.Errorf("a performance is running — stop it before changing characters")
	}
	mode := a.client().Engine.CurrentMode()
	if mode != "" && mode != "disable" {
		if !stopAutomation {
			return fmt.Errorf("Lua mode %q is active — stop it before changing characters", mode)
		}
		a.client().Engine.SetMode("disable", nil)
	}

	a.configMu.RLock()
	configured := a.cfg().CharacterProfileByID(profileID)
	if configured == nil {
		a.configMu.RUnlock()
		return fmt.Errorf("unknown character profile ID %q", profileID)
	}
	selected := cloneCharacterProfile(*configured)
	variables := a.cfg().ResolvedVariables(profileID)
	highPriority := a.cfg().ResolvedHighPriority(profileID)
	defaultID := a.cfg().CharacterProfiles.DefaultID
	a.configMu.RUnlock()

	a.characterMu.Lock()
	account := a.connectedAccount
	a.characterMu.Unlock()
	namespace := "character:" + selected.ID
	if selected.ID == defaultID {
		namespace = account
		if namespace == "" {
			namespace = "default"
		}
	}
	if err := a.client().Engine.SetPersistenceNamespace(namespace); err != nil {
		return fmt.Errorf("load Lua state for %s: %w", selected.Name, err)
	}

	a.characterMu.Lock()
	a.activeProfileID = selected.ID
	a.characterMu.Unlock()
	if a.client() != nil {
		a.client().SetInputVariables(variables)
		a.client().Engine.SetHighPriority(highPriority)
	}
	if a.deps.DesktopNotify != nil {
		a.deps.DesktopNotify.SetVariables(variables)
	}
	a.inputVariables.Store(cloneStringMap(variables))
	a.emit([]WireEvent{{Kind: KindCharacterProfile, CharacterProfile: &CharacterProfilePayload{
		ID: selected.ID, Name: selected.Name,
	}}})
	return nil
}

func (a *GuiApp) activateCharacterProfile(profileID string) {
	if err := a.SwitchCharacterProfile(profileID, false); err != nil {
		a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{
			Title: "Character switch blocked", Message: err.Error(),
		}}})
	}
}

func (a *GuiApp) activateCharacterProfileLocked(profileID string) {
	// The detected play/sleep command has already reached the game and cannot be
	// recalled. Stop an active Lua mode so Praetor follows the authoritative
	// character change instead of leaving its runtime attached to the old one.
	if err := a.switchCharacterProfileLocked(profileID, true); err != nil {
		a.emit([]WireEvent{{Kind: KindNotify, Notify: &NotifyPayload{
			Title: "Character switch blocked", Message: err.Error(),
		}}})
	}
}

func (a *GuiApp) activeCharacterProfileLocked() *config.CharacterProfileConfig {
	id := a.ActiveCharacterProfileID()
	for i := range a.cfg().CharacterProfiles.Profiles {
		if a.cfg().CharacterProfiles.Profiles[i].ID == id {
			return &a.cfg().CharacterProfiles.Profiles[i]
		}
	}
	return a.cfg().DefaultCharacterProfile()
}
