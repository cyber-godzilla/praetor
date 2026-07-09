package gui

import (
	"github.com/cyber-godzilla/praetor/internal/config"
)

// save persists the current config to disk. Returns any write error.
func (a *GuiApp) save() error {
	return config.Save(a.cfg(), a.deps.ConfigPath)
}

// ---------------------------------------------------------------------------
// Toggles (boolean settings). Each persists and applies to the live client.
// ---------------------------------------------------------------------------

// SetEchoTyped enables/disables echoing of user-typed commands.
func (a *GuiApp) SetEchoTyped(v bool) error {
	a.cfg().UI.EchoTyped = v
	a.client().Settings.EchoTyped = v
	return a.save()
}

// SetEchoScript enables/disables echoing of script-sent commands.
func (a *GuiApp) SetEchoScript(v bool) error {
	a.cfg().UI.EchoScript = v
	a.client().Settings.EchoScript = v
	return a.save()
}

// SetColorWords toggles color-word rendering (applied in the event loop).
func (a *GuiApp) SetColorWords(v bool) error {
	a.cfg().UI.ColorWords = v
	a.colorWords.Store(v)
	return a.save()
}

// SetHideIPs toggles IP masking in game text.
func (a *GuiApp) SetHideIPs(v bool) error {
	a.cfg().UI.HideIPs = v
	return a.save()
}

// SetSessionLogging toggles transcript logging.
func (a *GuiApp) SetSessionLogging(v bool) error {
	a.cfg().Logging.Session.Enabled = v
	return a.save()
}

// SetLogPath sets the session transcript directory.
func (a *GuiApp) SetLogPath(path string) error {
	a.cfg().Logging.Session.Path = path
	return a.save()
}

// SetDisplayMode persists the display mode (sidebar/topbar/off).
func (a *GuiApp) SetDisplayMode(mode string) error {
	a.cfg().UI.DisplayMode = mode
	return a.save()
}

// SetMinimapScale persists and applies the minimap scale.
func (a *GuiApp) SetMinimapScale(scale float64) error {
	a.cfg().UI.MinimapScale = scale
	a.render.setScale(scale)
	if err := a.save(); err != nil {
		return err
	}
	a.RefreshGraphics()
	return nil
}

// SetOutputFontSize persists the game-output text size in pixels. Applied in
// the frontend via CSS, so no re-render is needed here.
func (a *GuiApp) SetOutputFontSize(px int) error {
	a.cfg().UI.OutputFontSize = px
	return a.save()
}

// SetCRTEffects persists the three retro CRT effect toggles (scanlines, the
// rolling band, and the phosphor bloom). Applied in the frontend via CSS.
func (a *GuiApp) SetCRTEffects(scanlines, roll, bloom bool) error {
	a.cfg().UI.CRTScanlines = scanlines
	a.cfg().UI.CRTRoll = roll
	a.cfg().UI.CRTBloom = bloom
	return a.save()
}

// SetCompassScale persists the compass scale. The compass is rendered at a
// fixed size; the scale only affects on-screen display (handled in the
// frontend via CSS), so no re-render is needed here.
func (a *GuiApp) SetCompassScale(scale float64) error {
	a.cfg().UI.CompassScale = scale
	return a.save()
}

// ---------------------------------------------------------------------------
// List / structured settings.
// ---------------------------------------------------------------------------

// SetHighlights replaces the highlight rules.
func (a *GuiApp) SetHighlights(highlights []config.HighlightConfig) error {
	a.cfg().Highlights = highlights
	return a.save()
}

// SetCustomTabs replaces the custom tab definitions.
func (a *GuiApp) SetCustomTabs(tabs []config.CustomTabConfig) error {
	a.cfg().UI.CustomTabs = tabs
	return a.save()
}

// SetActionSets replaces the sidebar quick-action sets.
func (a *GuiApp) SetActionSets(sets []config.ActionSet) error {
	a.cfg().UI.ActionSets = sets
	return a.save()
}

// SetQuickCycleModes replaces the Alt+M quick-cycle mode list.
func (a *GuiApp) SetQuickCycleModes(modes []string) error {
	a.cfg().UI.QuickCycleModes = modes
	return a.save()
}

// SetHighPriority replaces the high-priority command list and applies it to
// the engine queue.
func (a *GuiApp) SetHighPriority(cmds []string) error {
	a.cfg().Commands.HighPriority = cmds
	a.client().Engine.SetHighPriority(cmds)
	return a.save()
}

// SetIgnoreOOC replaces the OOC ignorelist and applies it live.
func (a *GuiApp) SetIgnoreOOC(names []string) error {
	a.cfg().Ignorelist.OOC = names
	a.client().SetIgnoreOOC(names)
	return a.save()
}

// SetIgnoreThink replaces the Think ignorelist and applies it live.
func (a *GuiApp) SetIgnoreThink(names []string) error {
	a.cfg().Ignorelist.Think = names
	a.client().SetIgnoreThink(names)
	return a.save()
}

// SetNotifications replaces desktop notification settings and applies them.
func (a *GuiApp) SetNotifications(cfg config.DesktopNotificationsConfig) error {
	a.cfg().Notifications.Desktop = cfg
	a.deps.DesktopNotify.UpdateConfig(cfg)
	return a.save()
}

// SetScriptDirs replaces the Lua script directories and reloads modes from the
// new locations.
func (a *GuiApp) SetScriptDirs(dirs []string) error {
	a.cfg().Scripts = dirs
	if err := a.save(); err != nil {
		return err
	}
	expanded := make([]string, 0, len(dirs))
	for _, d := range dirs {
		expanded = append(expanded, expandPath(d))
	}
	a.deps.ScriptDirs = expanded
	return a.client().Engine.UpdateScriptDirs(expanded)
}
