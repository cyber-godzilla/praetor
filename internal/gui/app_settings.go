package gui

import (
	"github.com/cyber-godzilla/praetor/internal/config"
)

// withConfig runs mutate and persists the config, serialized under the facade
// lock. Wails dispatches each bound setter on its own goroutine, so without this
// a field write could race a concurrent Save's marshal of the whole struct.
// Do live-apply work that must not hold the lock (engine reloads, re-renders)
// after this returns, not inside mutate.
func (a *GuiApp) withConfig(mutate func()) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	mutate()
	return config.Save(a.cfg(), a.deps.ConfigPath)
}

// ---------------------------------------------------------------------------
// Toggles (boolean settings). Each persists and applies to the live client.
// ---------------------------------------------------------------------------

// SetEchoTyped enables/disables echoing of user-typed commands.
func (a *GuiApp) SetEchoTyped(v bool) error {
	return a.withConfig(func() {
		a.cfg().UI.EchoTyped = v
		a.client().Settings.EchoTyped = v
	})
}

// SetEchoScript enables/disables echoing of script-sent commands.
func (a *GuiApp) SetEchoScript(v bool) error {
	return a.withConfig(func() {
		a.cfg().UI.EchoScript = v
		a.client().Settings.EchoScript = v
	})
}

// SetColorWords toggles color-word rendering (applied in the event loop).
func (a *GuiApp) SetColorWords(v bool) error {
	return a.withConfig(func() {
		a.cfg().UI.ColorWords = v
		a.colorWords.Store(v)
	})
}

// SetHideIPs toggles IP masking in game text.
func (a *GuiApp) SetHideIPs(v bool) error {
	return a.withConfig(func() { a.cfg().UI.HideIPs = v })
}

// SetInputSpellcheck toggles the webview spellchecker on the command input.
// Applied live in the frontend via the input's spellcheck attribute.
func (a *GuiApp) SetInputSpellcheck(v bool) error {
	return a.withConfig(func() { a.cfg().UI.InputSpellcheck = v })
}

// SetUpdateCheck toggles the startup check for newer releases.
func (a *GuiApp) SetUpdateCheck(v bool) error {
	return a.withConfig(func() { a.cfg().Updates.Check = v })
}

// SetSessionLogging toggles transcript logging.
func (a *GuiApp) SetSessionLogging(v bool) error {
	return a.withConfig(func() { a.cfg().Logging.Session.Enabled = v })
}

// SetLogPath sets the session transcript directory.
func (a *GuiApp) SetLogPath(path string) error {
	return a.withConfig(func() { a.cfg().Logging.Session.Path = path })
}

// SetDisplayMode persists the display mode (sidebar/topbar/off).
func (a *GuiApp) SetDisplayMode(mode string) error {
	return a.withConfig(func() { a.cfg().UI.DisplayMode = mode })
}

// SetNumpadNavigation persists the numpad-navigation mode (numlock/always/off).
// The value is validated on the next load; the frontend applies it live.
func (a *GuiApp) SetNumpadNavigation(mode string) error {
	return a.withConfig(func() { a.cfg().UI.NumpadNavigation = mode })
}

// SetMinimapScale persists and applies the minimap scale.
func (a *GuiApp) SetMinimapScale(scale float64) error {
	if err := a.withConfig(func() {
		a.cfg().UI.MinimapScale = scale
		a.render.setScale(scale)
	}); err != nil {
		return err
	}
	a.RefreshGraphics()
	return nil
}

// SetOutputFontSize persists the game-output text size in pixels. Applied in
// the frontend via CSS, so no re-render is needed here.
func (a *GuiApp) SetOutputFontSize(px int) error {
	return a.withConfig(func() { a.cfg().UI.OutputFontSize = px })
}

// SetCRTEffects persists the three retro CRT effect toggles (scanlines, the
// rolling band, and the phosphor bloom). Applied in the frontend via CSS.
func (a *GuiApp) SetCRTEffects(scanlines, roll, bloom bool) error {
	return a.withConfig(func() {
		a.cfg().UI.CRTScanlines = scanlines
		a.cfg().UI.CRTRoll = roll
		a.cfg().UI.CRTBloom = bloom
	})
}

// SetCompassScale persists the compass scale. The compass is rendered at a
// fixed size; the scale only affects on-screen display (handled in the
// frontend via CSS), so no re-render is needed here.
func (a *GuiApp) SetCompassScale(scale float64) error {
	return a.withConfig(func() { a.cfg().UI.CompassScale = scale })
}

// ---------------------------------------------------------------------------
// List / structured settings.
// ---------------------------------------------------------------------------

// SetHighlights replaces the highlight rules.
func (a *GuiApp) SetHighlights(highlights []config.HighlightConfig) error {
	return a.withConfig(func() { a.cfg().Highlights = highlights })
}

// SetCustomTabs replaces the custom tab definitions.
func (a *GuiApp) SetCustomTabs(tabs []config.CustomTabConfig) error {
	return a.withConfig(func() { a.cfg().UI.CustomTabs = tabs })
}

// SetActionSets replaces the sidebar quick-action sets.
func (a *GuiApp) SetActionSets(sets []config.ActionSet) error {
	return a.withConfig(func() { a.cfg().UI.ActionSets = sets })
}

// SetQuickCycleModes replaces the Alt+M quick-cycle mode list.
func (a *GuiApp) SetQuickCycleModes(modes []string) error {
	return a.withConfig(func() { a.cfg().UI.QuickCycleModes = modes })
}

// SetHighPriority replaces the high-priority command list and applies it to
// the engine queue.
func (a *GuiApp) SetHighPriority(cmds []string) error {
	return a.withConfig(func() {
		a.cfg().Commands.HighPriority = cmds
		a.client().Engine.SetHighPriority(cmds)
	})
}

// SetIgnoreOOC replaces the OOC ignorelist and applies it live.
func (a *GuiApp) SetIgnoreOOC(names []string) error {
	return a.withConfig(func() {
		a.cfg().Ignorelist.OOC = names
		a.client().SetIgnoreOOC(names)
	})
}

// SetIgnoreThink replaces the Think ignorelist and applies it live.
func (a *GuiApp) SetIgnoreThink(names []string) error {
	return a.withConfig(func() {
		a.cfg().Ignorelist.Think = names
		a.client().SetIgnoreThink(names)
	})
}

// SetNotifications replaces desktop notification settings and applies them.
func (a *GuiApp) SetNotifications(cfg config.DesktopNotificationsConfig) error {
	return a.withConfig(func() {
		a.cfg().Notifications.Desktop = cfg
		a.deps.DesktopNotify.UpdateConfig(cfg)
	})
}

// SetScriptDirs replaces the Lua script directories and reloads modes from the
// new locations.
func (a *GuiApp) SetScriptDirs(dirs []string) error {
	if err := a.withConfig(func() { a.cfg().Scripts = dirs }); err != nil {
		return err
	}
	expanded := make([]string, 0, len(dirs))
	for _, d := range dirs {
		expanded = append(expanded, expandPath(d))
	}
	a.deps.ScriptDirs = expanded
	return a.client().Engine.UpdateScriptDirs(expanded)
}
