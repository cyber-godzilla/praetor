package gui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/colorwords"
	"github.com/cyber-godzilla/praetor/internal/config"
)

// withConfig serializes a mutation and persists it through the rollback-aware
// save helper used by the web and native frontends.
func (a *GuiApp) withConfig(mutate func()) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	mutate()
	return a.save()
}

// withConfigApplied keeps persistence and live application in the same
// serialized transaction. The live subsystem is updated only after the config
// has been saved successfully, so a failed save needs to roll back config only.
func (a *GuiApp) withConfigApplied(mutate, apply func()) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	mutate()
	if err := a.save(); err != nil {
		return err
	}
	apply()
	return nil
}

// save persists the current config to disk. Returns any write error.
func (a *GuiApp) save() error {
	if err := config.Save(a.cfg(), a.deps.ConfigPath); err != nil {
		if a.savedConfig != nil {
			*a.cfg() = *cloneConfig(a.savedConfig)
		}
		return err
	}
	a.savedConfig = cloneConfig(a.cfg())
	return nil
}

// ---------------------------------------------------------------------------
// Toggles (boolean settings). Each persists and applies to the live client.
// ---------------------------------------------------------------------------

// SetEchoTyped enables/disables echoing of user-typed commands.
func (a *GuiApp) SetEchoTyped(v bool) error {
	return a.withConfigApplied(
		func() { a.cfg().UI.EchoTyped = v },
		func() { a.client().SetEchoTyped(v) },
	)
}

// SetEchoScript enables/disables echoing of script-sent commands.
func (a *GuiApp) SetEchoScript(v bool) error {
	return a.withConfigApplied(
		func() { a.cfg().UI.EchoScript = v },
		func() { a.client().SetEchoScript(v) },
	)
}

// SetColorWords toggles color-word rendering (applied in the event loop).
func (a *GuiApp) SetColorWords(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.colorWords.Load()
	a.cfg().UI.ColorWords = v
	a.colorWords.Store(v)
	if err := a.save(); err != nil {
		a.colorWords.Store(old)
		return err
	}
	return nil
}

// SetHideIPs toggles IP masking in game text.
func (a *GuiApp) SetHideIPs(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.HideIPs = v
	return a.save()
}

// SetInputSpellcheck toggles the webview spellchecker on the command input.
// Applied live in the frontend via the input's spellcheck attribute.
func (a *GuiApp) SetInputSpellcheck(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.InputSpellcheck = v
	return a.save()
}

// SetKeepInputOnSend toggles keeping the sent command in the input (selected)
// instead of clearing it. Applied live in the frontend's submit path.
func (a *GuiApp) SetKeepInputOnSend(v bool) error {
	return a.withConfig(func() { a.cfg().UI.KeepInputOnSend = v })
}

// SetSemicolonDelay persists and applies the delay for newly submitted ;;
// command chains.
func (a *GuiApp) SetSemicolonDelay(ms int) error {
	if ms < config.MinSemicolonDelayMS || ms > config.MaxSemicolonDelayMS {
		return fmt.Errorf(";; delay must be between %d and %d milliseconds", config.MinSemicolonDelayMS, config.MaxSemicolonDelayMS)
	}
	return a.withConfigApplied(
		func() { a.cfg().Commands.SemicolonDelayMS = ms },
		func() { a.client().SetSemicolonDelay(time.Duration(ms) * time.Millisecond) },
	)
}

// SetUnbusyDelay persists and applies the delay after an &&-advancing response
// for newly submitted command chains.
func (a *GuiApp) SetUnbusyDelay(ms int) error {
	if ms < config.MinUnbusyDelayMS || ms > config.MaxUnbusyDelayMS {
		return fmt.Errorf("&& response delay must be between %d and %d milliseconds", config.MinUnbusyDelayMS, config.MaxUnbusyDelayMS)
	}
	return a.withConfigApplied(
		func() { a.cfg().Commands.UnbusyDelayMS = ms },
		func() { a.client().SetUnbusyDelay(time.Duration(ms) * time.Millisecond) },
	)
}

// SetUpdateCheck toggles the startup check for newer releases.
func (a *GuiApp) SetUpdateCheck(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().Updates.Check = v
	return a.save()
}

// SetMobileShowToolbar controls the Actions / Modes / Menu row in the mobile
// web dock. Native shells persist the shared value but do not apply it.
func (a *GuiApp) SetMobileShowToolbar(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.MobileShowToolbar = v
	return a.save()
}

// SetMobileShowTabBar controls the tab selector in the mobile-width web
// layout. When hidden, the web frontend keeps Menu available in the status bar.
func (a *GuiApp) SetMobileShowTabBar(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.MobileShowTabBar = v
	return a.save()
}

// SetMobileHideNavigationOnInput controls whether the mobile web map and
// compass are hidden while the command field owns focus.
func (a *GuiApp) SetMobileHideNavigationOnInput(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.MobileHideNavigationOnInput = v
	return a.save()
}

// SetMobileLowercaseFirstLetter controls the mobile web command-input
// safeguard for software keyboards that capitalize each new command.
func (a *GuiApp) SetMobileLowercaseFirstLetter(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.MobileLowercaseFirstLetter = v
	return a.save()
}

// SetMobileOutputFontSize persists the game-output size used by mobile-width
// web layouts. Desktop web and native Wails continue to use OutputFontSize.
func (a *GuiApp) SetMobileOutputFontSize(px int) error {
	if px < 6 || px > 40 {
		return fmt.Errorf("mobile output font size must be between 6 and 40")
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.MobileOutputFontSize = px
	return a.save()
}

// SetRetainAppLogs selects permanent timestamped application-log archives.
// The active writer cannot be replaced safely while other goroutines are
// logging, so the new policy takes effect the next time Praetor starts.
func (a *GuiApp) SetRetainAppLogs(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().Logging.App.Retain = v
	return a.save()
}

// SetSessionLogging toggles transcript logging.
func (a *GuiApp) SetSessionLogging(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.cfg().Logging.Session.Enabled
	path := a.sessionLogDir(a.cfg().Logging.Session.Path)
	if a.deps.SessionLog != nil {
		if err := a.deps.SessionLog.Reconfigure(v, path); err != nil {
			return err
		}
	}
	a.cfg().Logging.Session.Enabled = v
	if err := a.save(); err != nil {
		a.cfg().Logging.Session.Enabled = old
		if a.deps.SessionLog != nil {
			_ = a.deps.SessionLog.Reconfigure(old, path)
		}
		return err
	}
	return nil
}

// SetLogPath sets the session transcript directory.
func (a *GuiApp) SetLogPath(path string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.cfg().Logging.Session.Path
	enabled := a.cfg().Logging.Session.Enabled
	if a.deps.SessionLog != nil {
		if err := a.deps.SessionLog.Reconfigure(enabled, a.sessionLogDir(path)); err != nil {
			return err
		}
	}
	a.cfg().Logging.Session.Path = path
	if err := a.save(); err != nil {
		a.cfg().Logging.Session.Path = old
		if a.deps.SessionLog != nil {
			_ = a.deps.SessionLog.Reconfigure(enabled, a.sessionLogDir(old))
		}
		return err
	}
	return nil
}

func (a *GuiApp) sessionLogDir(path string) string {
	if path == "" {
		return a.deps.SessionsDir
	}
	return expandPath(path)
}

// SetDisplayMode persists the display mode (sidebar/topbar/off).
func (a *GuiApp) SetDisplayMode(mode string) error {
	if mode != "sidebar" && mode != "topbar" && mode != "off" {
		return fmt.Errorf("display mode must be sidebar, topbar, or off")
	}
	return a.withConfig(func() { a.cfg().UI.DisplayMode = mode })
}

// SetGUISidebarsVisible persists the GUI/web global sidebar toggle without
// rewriting the TUI's independent sidebar/topbar preference.
func (a *GuiApp) SetGUISidebarsVisible(visible bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().DisplayProfileForCharacter(a.ActiveCharacterProfileID()).SidebarsVisible = visible
	return a.save()
}

// SetNumpadNavigation persists the numpad-navigation mode (numlock/always/off).
// The value is validated on the next load; the frontend applies it live.
func (a *GuiApp) SetNumpadNavigation(mode string) error {
	if mode != "numlock" && mode != "always" && mode != "off" {
		return fmt.Errorf("numpad navigation must be numlock, always, or off")
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.NumpadNavigation = mode
	return a.save()
}

// SetMinimapScale persists and applies the minimap scale.
func (a *GuiApp) SetMinimapScale(scale float64) error {
	if scale < 0.2 || scale > 3 {
		return fmt.Errorf("minimap scale must be between 0.2 and 3")
	}
	return a.withConfigApplied(
		func() { a.cfg().UI.MinimapScale = scale },
		func() {
			a.render.setScale(scale)
			a.refreshGraphics()
		},
	)
}

// SetOutputFontSize persists the game-output text size in pixels. Applied in
// the frontend via CSS, so no re-render is needed here.
func (a *GuiApp) SetOutputFontSize(px int) error {
	if px < 8 || px > 40 {
		return fmt.Errorf("output font size must be between 8 and 40")
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.OutputFontSize = px
	return a.save()
}

// SetGUILayout persists the browser-native pixel dimensions used by the GUI.
// The TUI's column/row settings remain independent because they are not
// meaningful CSS dimensions.
func (a *GuiApp) SetGUILayout(sidebarWidth, minimapHeight int) error {
	if sidebarWidth < 180 || sidebarWidth > 600 {
		return fmt.Errorf("GUI sidebar width must be between 180 and 600 pixels")
	}
	if minimapHeight < 80 || minimapHeight > 400 {
		return fmt.Errorf("GUI minimap height must be between 80 and 400 pixels")
	}
	return a.withConfig(func() {
		a.cfg().UI.GUISidebarWidth = sidebarWidth
		a.cfg().UI.GUIMinimapHeight = minimapHeight
	})
}

// SetCRTEffects persists the three retro CRT effect toggles (scanlines, the
// rolling band, and the phosphor bloom). Applied in the frontend via CSS.
func (a *GuiApp) SetCRTEffects(scanlines, roll, bloom bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.CRTScanlines = scanlines
	a.cfg().UI.CRTRoll = roll
	a.cfg().UI.CRTBloom = bloom
	return a.save()
}

// SetCompassScale persists the compass scale. The compass is rendered at a
// fixed size; the scale only affects on-screen display (handled in the
// frontend via CSS), so no re-render is needed here.
func (a *GuiApp) SetCompassScale(scale float64) error {
	if scale < 0.5 || scale > 3 {
		return fmt.Errorf("compass scale must be between 0.5 and 3")
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.CompassScale = scale
	return a.save()
}

// ---------------------------------------------------------------------------
// List / structured settings.
// ---------------------------------------------------------------------------

// SetHighlights replaces the highlight rules.
func (a *GuiApp) SetHighlights(highlights []config.HighlightConfig) error {
	normalized := make([]config.HighlightConfig, len(highlights))
	for i, highlight := range highlights {
		highlight.Pattern = strings.TrimSpace(highlight.Pattern)
		if highlight.Pattern == "" {
			return fmt.Errorf("highlight patterns must not be empty")
		}
		style, ok := colorwords.LookupStyle(highlight.Style)
		if !ok {
			return fmt.Errorf("invalid highlight style %q", highlight.Style)
		}
		highlight.Style = style.Name
		normalized[i] = highlight
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.activeCharacterProfileLocked().Highlights = normalized
	return a.save()
}

// SetSpeechColors replaces the character speech-color rules and atomically
// publishes the persisted snapshot to the event rendering loop.
func (a *GuiApp) SetSpeechColors(rules []config.SpeechColorConfig) error {
	normalized := make([]config.SpeechColorConfig, 0, len(rules))
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		rule.Character = strings.TrimSpace(rule.Character)
		if rule.Character == "" {
			return fmt.Errorf("speech color character patterns must not be empty")
		}
		key := strings.ToLower(rule.Character)
		if seen[key] {
			return fmt.Errorf("duplicate speech color character pattern %q", rule.Character)
		}
		style, ok := colorwords.LookupStyle(rule.Style)
		if !ok {
			return fmt.Errorf("invalid Colorword style %q", rule.Style)
		}
		seen[key] = true
		rule.Style = style.Name
		normalized = append(normalized, rule)
	}

	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().SpeechColors = cloneSpeechColors(normalized)
	if err := a.save(); err != nil {
		return err
	}
	a.speechColors.Store(cloneSpeechColors(normalized))
	return nil
}

// SetWoundHighlights persists and atomically publishes wound styling so the
// event loop applies changes without a reconnect or restart.
func (a *GuiApp) SetWoundHighlights(cfg config.WoundHighlightConfig) error {
	valid := map[string]bool{"red": true, "purple": true, "blue": true, "orange": true}
	for category, family := range map[string]string{
		"cuts": cfg.Cuts, "bruises": cfg.Bruises, "punctures": cfg.Punctures, "burns": cfg.Burns,
	} {
		if !valid[family] {
			return fmt.Errorf("invalid %s wound color family %q", category, family)
		}
	}
	return a.withConfigApplied(
		func() { a.cfg().WoundHighlights = cfg },
		func() { a.woundHighlights.Store(cfg) },
	)
}

// SetCustomTabs replaces the custom tab definitions.
func (a *GuiApp) SetCustomTabs(tabs []config.CustomTabConfig) error {
	for _, tab := range tabs {
		if strings.TrimSpace(tab.Name) == "" {
			return fmt.Errorf("custom tab names must not be empty")
		}
	}
	tabs = config.NormalizeCustomViewIDs(tabs)
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.activeCharacterProfileLocked().CustomViews = tabs
	a.cfg().PlaceUnplacedCustomViewsInDefault()
	if err := a.cfg().Validate(); err != nil {
		return err
	}
	return a.save()
}

// SetViewLayout atomically replaces the unified GUI/web placement model and
// its Custom View definitions. Editors stage both together so a placement can
// never refer to a view that was only half-created or already deleted.
func (a *GuiApp) SetViewLayout(layout config.DisplayProfileConfig, custom []config.CustomTabConfig) error {
	return a.SetProfileViewLayout(a.ActiveCharacterProfileID(), layout, custom)
}

// SetProfileViewLayout updates any character-owned view collection from the
// global View Library. SetViewLayout remains the active-character convenience
// binding used by the Layout editor.
func (a *GuiApp) SetProfileViewLayout(profileID string, layout config.DisplayProfileConfig, custom []config.CustomTabConfig) error {
	seenNames := make(map[string]bool, len(custom))
	for _, view := range custom {
		name := strings.TrimSpace(view.Name)
		if name == "" {
			return fmt.Errorf("custom view names must not be empty")
		}
		key := strings.ToLower(name)
		if seenNames[key] {
			return fmt.Errorf("duplicate custom view name %q", name)
		}
		seenNames[key] = true
	}
	custom = config.NormalizeCustomViewIDs(custom)
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.cfg().CharacterProfileByID(profileID)
	current := a.cfg().DisplayProfileForCharacter(profileID)
	if profile == nil || current == nil {
		return fmt.Errorf("character profile %q does not exist", profileID)
	}
	profile.CustomViews = custom
	layout.ID = current.ID
	layout.Name = current.Name
	layout.CharacterProfileID = current.CharacterProfileID
	*current = layout
	if err := a.cfg().Validate(); err != nil {
		return err
	}
	return a.save()
}

// SetActionSets replaces the sidebar quick-action sets.
func (a *GuiApp) SetActionSets(sets []config.ActionSet) error {
	sets = config.NormalizeActionSetIDs(sets)
	if sets == nil {
		sets = []config.ActionSet{}
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	profile.ActionSetsMode = config.CharacterSettingCustomize
	// Action Set UUIDs make selection and membership stable; they do not make
	// the mutable definition global. A character that starts by copying an
	// inherited set may retain its ID, but editing that character must never
	// rewrite another profile's copy.
	profile.ActionSets = sets
	for profileIndex := range a.cfg().CharacterProfiles.Profiles {
		candidate := &a.cfg().CharacterProfiles.Profiles[profileIndex]
		resolved := a.cfg().ResolvedActionSets(candidate.ID)
		found := false
		for _, set := range resolved {
			if set.ID == candidate.SelectedActionSetID {
				found = true
				break
			}
		}
		if !found {
			candidate.SelectedActionSetID = ""
			if len(resolved) > 0 {
				candidate.SelectedActionSetID = resolved[0].ID
			}
		}
	}
	return a.save()
}

// SetSelectedActionSetID persists the current Action Set by stable identity so
// rename and reorder do not change the player's selection.
func (a *GuiApp) SetSelectedActionSetID(id string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	for _, set := range a.cfg().ResolvedActionSets(profile.ID) {
		if set.ID == id {
			profile.SelectedActionSetID = id
			return a.save()
		}
	}
	return fmt.Errorf("unknown action set ID %q", id)
}

// SetQuickCycleModes replaces the Alt+M quick-cycle mode list.
func (a *GuiApp) SetQuickCycleModes(modes []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	if profile.ID == a.cfg().CharacterProfiles.DefaultID {
		a.cfg().UI.QuickCycleModes = append([]string(nil), modes...)
	} else {
		profile.QuickCycleMode = config.CharacterSettingCustomize
		profile.QuickCycleModes = append([]string{}, modes...)
	}
	return a.save()
}

// SetQuickCycleInheritance switches the active character between live Default
// values and a character-owned list. Customizing begins with the effective
// list so the transition is lossless.
func (a *GuiApp) SetQuickCycleInheritance(inherit bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	if profile.ID == a.cfg().CharacterProfiles.DefaultID {
		return fmt.Errorf("the Default profile cannot inherit quick-cycle modes")
	}
	if inherit {
		profile.QuickCycleMode = config.CharacterSettingInherit
	} else {
		profile.QuickCycleModes = a.cfg().ResolvedQuickCycleModes(profile.ID)
		profile.QuickCycleMode = config.CharacterSettingCustomize
	}
	return a.save()
}

// SetHighPriority replaces the high-priority command list and applies it to
// the engine queue.
func (a *GuiApp) SetHighPriority(cmds []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	old := a.cfg().ResolvedHighPriority(profile.ID)
	if profile.ID == a.cfg().CharacterProfiles.DefaultID {
		a.cfg().Commands.HighPriority = append([]string(nil), cmds...)
	} else {
		profile.PriorityCommandsMode = config.CharacterSettingCustomize
		profile.HighPriority = append([]string{}, cmds...)
	}
	a.client().Engine.SetHighPriority(cmds)
	if err := a.save(); err != nil {
		a.client().Engine.SetHighPriority(old)
		return err
	}
	return nil
}

// SetHighPriorityInheritance changes whether the active character follows the
// Default priority-command list.
func (a *GuiApp) SetHighPriorityInheritance(inherit bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	if profile.ID == a.cfg().CharacterProfiles.DefaultID {
		return fmt.Errorf("the Default profile cannot inherit priority commands")
	}
	old := a.cfg().ResolvedHighPriority(profile.ID)
	if inherit {
		profile.PriorityCommandsMode = config.CharacterSettingInherit
	} else {
		profile.HighPriority = a.cfg().ResolvedHighPriority(profile.ID)
		profile.PriorityCommandsMode = config.CharacterSettingCustomize
	}
	resolved := a.cfg().ResolvedHighPriority(profile.ID)
	a.client().Engine.SetHighPriority(resolved)
	if err := a.save(); err != nil {
		a.client().Engine.SetHighPriority(old)
		return err
	}
	return nil
}

// SetActionSetInheritance changes whether the active character follows the
// Default Action Set collection.
func (a *GuiApp) SetActionSetInheritance(inherit bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	if profile.ID == a.cfg().CharacterProfiles.DefaultID {
		return fmt.Errorf("the Default profile cannot inherit Action Sets")
	}
	if inherit {
		profile.ActionSetsMode = config.CharacterSettingInherit
	} else {
		profile.ActionSets = a.cfg().ResolvedActionSets(profile.ID)
		profile.ActionSetsMode = config.CharacterSettingCustomize
	}
	profile.SelectedActionSetID = ""
	resolved := a.cfg().ResolvedActionSets(profile.ID)
	if len(resolved) > 0 {
		profile.SelectedActionSetID = resolved[0].ID
	}
	return a.save()
}

// DeleteActionSetEverywhere removes every profile copy with the stable Action
// Set ID. Ordinary removal in SetActionSets affects only the active profile.
func (a *GuiApp) DeleteActionSetEverywhere(id string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	found := false
	for profileIndex := range a.cfg().CharacterProfiles.Profiles {
		profile := &a.cfg().CharacterProfiles.Profiles[profileIndex]
		kept := profile.ActionSets[:0]
		for _, set := range profile.ActionSets {
			if set.ID == id {
				found = true
				continue
			}
			kept = append(kept, set)
		}
		profile.ActionSets = kept
		if profile.SelectedActionSetID == id {
			profile.SelectedActionSetID = ""
			resolved := a.cfg().ResolvedActionSets(profile.ID)
			if len(resolved) > 0 {
				profile.SelectedActionSetID = resolved[0].ID
			}
		}
	}
	if !found {
		return fmt.Errorf("unknown Action Set ID %q", id)
	}
	return a.save()
}

// SetInputVariables replaces the name/value map used for ${name} and
// ${name:fallback} substitutions in typed command-line input and applies it
// live.
func (a *GuiApp) SetInputVariables(variables map[string]string) error {
	for name := range variables {
		if !config.ValidVariableName(name) {
			return fmt.Errorf("invalid variable name %q", name)
		}
	}
	cloned := make(map[string]string, len(variables))
	for name, value := range variables {
		cloned[name] = value
	}
	var resolved map[string]string
	return a.withConfigApplied(
		func() {
			profile := a.activeCharacterProfileLocked()
			profile.Variables = cloned
			resolved = a.cfg().ResolvedVariables(profile.ID)
		},
		func() {
			a.client().SetInputVariables(resolved)
			if a.deps.DesktopNotify != nil {
				a.deps.DesktopNotify.SetVariables(resolved)
			}
			a.inputVariables.Store(cloneStringMap(resolved))
		},
	)
}

// persistResolvedInputVariables stores the client's full effective snapshot
// (used by /set-variable and PraetorScript directives) as the active
// character's independent variable list.
func (a *GuiApp) persistResolvedInputVariables(variables map[string]string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	profile := a.activeCharacterProfileLocked()
	profile.Variables = cloneStringMap(variables)
	if err := a.save(); err != nil {
		return err
	}
	if a.deps.DesktopNotify != nil {
		a.deps.DesktopNotify.SetVariables(variables)
	}
	a.inputVariables.Store(cloneStringMap(variables))
	return nil
}

// SetIgnoreOOC replaces the OOC ignorelist and applies it live.
func (a *GuiApp) SetIgnoreOOC(names []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := append([]string(nil), a.cfg().Ignorelist.OOC...)
	a.cfg().Ignorelist.OOC = names
	a.client().SetIgnoreOOC(names)
	if err := a.save(); err != nil {
		a.client().SetIgnoreOOC(old)
		return err
	}
	return nil
}

// SetIgnoreThink replaces the Think ignorelist and applies it live.
func (a *GuiApp) SetIgnoreThink(names []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := append([]string(nil), a.cfg().Ignorelist.Think...)
	a.cfg().Ignorelist.Think = names
	a.client().SetIgnoreThink(names)
	if err := a.save(); err != nil {
		a.client().SetIgnoreThink(old)
		return err
	}
	return nil
}

// SetNotifications replaces all notification destinations atomically and
// applies them live.
func (a *GuiApp) SetNotifications(cfg config.NotificationsConfig) error {
	if cfg.Desktop.HealthBelow.Threshold < 0 || cfg.Desktop.HealthBelow.Threshold > 100 ||
		cfg.Desktop.FatigueBelow.Threshold < 0 || cfg.Desktop.FatigueBelow.Threshold > 100 {
		return fmt.Errorf("notification thresholds must be between 0 and 100")
	}
	cfg.Discord.WebhookURL = strings.TrimSpace(cfg.Discord.WebhookURL)
	if cfg.Discord.Enabled && cfg.Discord.WebhookURL == "" {
		return fmt.Errorf("a Discord webhook URL is required when Discord notifications are enabled")
	}
	if cfg.Discord.WebhookURL != "" {
		if err := client.ValidateDiscordWebhookURL(cfg.Discord.WebhookURL); err != nil {
			return err
		}
	}
	cfg.Normalize()
	return a.withConfigApplied(
		func() { a.cfg().Notifications = cfg },
		func() {
			if a.deps.Client != nil {
				a.client().SetScriptNotificationPreferences(cfg.Desktop.AllowScriptNotifications, cfg.Desktop.ScriptSound)
			}
			if a.deps.DesktopNotify != nil {
				a.deps.DesktopNotify.UpdateConfig(cfg.Desktop)
			}
			a.applyDiscordNotifications(cfg.Discord)
		},
	)
}

// SetWindowFocused updates focus-aware native notification rules. Web clients
// make this decision per browser from the options carried on each event.
func (a *GuiApp) SetWindowFocused(focused bool) {
	if a.deps.DesktopNotify != nil {
		a.deps.DesktopNotify.SetFocused(focused)
	}
}

func (a *GuiApp) applyDiscordNotifications(cfg config.DiscordNotificationsConfig) {
	if a.deps.DiscordNotify == nil {
		a.deps.DiscordNotify = client.NewDiscordNotifier(cfg)
		if a.deps.DesktopNotify != nil {
			a.deps.DesktopNotify.SetDiscordNotifier(a.deps.DiscordNotify)
		}
		if a.deps.Client != nil {
			a.deps.Client.SetDiscordNotifier(a.deps.DiscordNotify)
		}
		return
	}
	a.deps.DiscordNotify.UpdateConfig(cfg)
}

// TestDiscordNotification performs a bounded synchronous delivery so the UI
// can report success or failure directly to the player.
func (a *GuiApp) TestDiscordNotification() error {
	if a.deps.DiscordNotify == nil {
		return fmt.Errorf("Discord notifications are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	return a.deps.DiscordNotify.SendTest(ctx)
}

// SetScriptDirs replaces the Lua script directories and reloads modes from the
// new locations.
func (a *GuiApp) SetScriptDirs(dirs []string) error {
	a.configMu.Lock()
	expanded := make([]string, 0, len(dirs))
	for _, d := range dirs {
		expanded = append(expanded, expandPath(d))
	}
	oldDirs := append([]string(nil), a.cfg().Scripts...)
	oldDisabled := append([]string(nil), a.cfg().DisabledScripts...)
	oldExpanded := append([]string(nil), a.deps.ScriptDirs...)
	oldRuntime := a.client().Engine.ScriptSources()
	a.cfg().Scripts = append([]string(nil), dirs...)
	dirKeys := make(map[string]bool, len(dirs))
	for _, dir := range dirs {
		dirKeys[localScriptDirectoryKey(dir)] = true
	}
	disabled := make([]string, 0, len(a.cfg().DisabledScripts))
	for _, dir := range a.cfg().DisabledScripts {
		if dirKeys[localScriptDirectoryKey(dir)] {
			disabled = append(disabled, dir)
		}
	}
	a.cfg().DisabledScripts = disabled
	a.deps.ScriptDirs = expanded
	nextRuntime := a.scriptGraphSourcesLocked()
	a.configMu.Unlock()

	// Mode files execute while they load and may persist a variable through the
	// GUI callback. Keep configuration unlocked during that execution.
	if err := a.client().Engine.UpdateScriptSources(nextRuntime); err != nil {
		a.configMu.Lock()
		a.cfg().Scripts = oldDirs
		a.cfg().DisabledScripts = oldDisabled
		a.deps.ScriptDirs = oldExpanded
		a.configMu.Unlock()
		return err
	}
	a.configMu.Lock()
	if err := a.save(); err != nil {
		a.cfg().Scripts = oldDirs
		a.cfg().DisabledScripts = oldDisabled
		a.deps.ScriptDirs = oldExpanded
		a.configMu.Unlock()
		_ = a.client().Engine.UpdateScriptSources(oldRuntime)
		return err
	}
	a.configMu.Unlock()
	return nil
}
