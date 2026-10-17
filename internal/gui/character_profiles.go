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
	a.configMu.Lock()
	existing := make(map[string]config.CharacterProfileConfig, len(a.cfg().CharacterProfiles.Profiles))
	for _, profile := range a.cfg().CharacterProfiles.Profiles {
		existing[profile.ID] = profile
	}
	defaultProfile := cloneCharacterProfile(*a.cfg().DefaultCharacterProfile())
	replacement := make([]config.CharacterProfileConfig, 0, len(identities))
	for _, identity := range identities {
		profile, ok := existing[identity.ID]
		if !ok {
			profile = cloneCharacterProfile(defaultProfile)
			profile.ID = identity.ID
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
		a.configMu.Unlock()
		return err
	}
	if err := a.save(); err != nil {
		a.configMu.Unlock()
		return err
	}
	a.configMu.Unlock()

	if !seenIDs[strings.ToLower(activeID)] {
		a.activateCharacterProfile(defaultID)
	}
	return nil
}

func cloneCharacterProfile(source config.CharacterProfileConfig) config.CharacterProfileConfig {
	cloned := source
	cloned.Variables = cloneStringMap(source.Variables)
	cloned.CustomViews = append([]config.CustomTabConfig(nil), source.CustomViews...)
	for i := range cloned.CustomViews {
		cloned.CustomViews[i].Rules = append([]config.TabRuleConfig(nil), source.CustomViews[i].Rules...)
	}
	cloned.ActionSets = append([]config.ActionSet(nil), source.ActionSets...)
	for i := range cloned.ActionSets {
		cloned.ActionSets[i].Buttons = append([]config.ActionButton(nil), source.ActionSets[i].Buttons...)
	}
	return cloned
}

func (a *GuiApp) observeCharacterSelection(input string) {
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
	a.activateCharacterProfile(profileID)
}

func (a *GuiApp) activateCharacterProfile(profileID string) {
	a.configMu.RLock()
	var selected *config.CharacterProfileConfig
	for i := range a.cfg().CharacterProfiles.Profiles {
		if a.cfg().CharacterProfiles.Profiles[i].ID == profileID {
			profile := cloneCharacterProfile(a.cfg().CharacterProfiles.Profiles[i])
			selected = &profile
			break
		}
	}
	a.configMu.RUnlock()
	if selected == nil {
		return
	}

	a.characterMu.Lock()
	a.activeProfileID = selected.ID
	a.characterMu.Unlock()
	if a.client() != nil {
		a.client().SetInputVariables(selected.Variables)
	}
	if a.deps.DesktopNotify != nil {
		a.deps.DesktopNotify.SetVariables(selected.Variables)
	}
	a.inputVariables.Store(cloneStringMap(selected.Variables))
	a.emit([]WireEvent{{Kind: KindCharacterProfile, CharacterProfile: &CharacterProfilePayload{
		ID: selected.ID, Name: selected.Name,
	}}})
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
