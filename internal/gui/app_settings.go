package gui

import (
	"fmt"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/config"
)

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
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.client().Settings.EchoTyped
	a.cfg().UI.EchoTyped = v
	a.client().Settings.EchoTyped = v
	if err := a.save(); err != nil {
		a.client().Settings.EchoTyped = old
		return err
	}
	return nil
}

// SetEchoScript enables/disables echoing of script-sent commands.
func (a *GuiApp) SetEchoScript(v bool) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.client().Settings.EchoScript
	a.cfg().UI.EchoScript = v
	a.client().Settings.EchoScript = v
	if err := a.save(); err != nil {
		a.client().Settings.EchoScript = old
		return err
	}
	return nil
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
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.DisplayMode = mode
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
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.cfg().UI.MinimapScale
	a.cfg().UI.MinimapScale = scale
	a.render.setScale(scale)
	if err := a.save(); err != nil {
		a.render.setScale(old)
		return err
	}
	a.RefreshGraphics()
	return nil
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
	validStyles := map[string]bool{"red": true, "gold": true, "green": true, "blue": true}
	for _, highlight := range highlights {
		if !validStyles[highlight.Style] {
			return fmt.Errorf("invalid highlight style %q", highlight.Style)
		}
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().Highlights = highlights
	return a.save()
}

// SetCustomTabs replaces the custom tab definitions.
func (a *GuiApp) SetCustomTabs(tabs []config.CustomTabConfig) error {
	for _, tab := range tabs {
		if strings.TrimSpace(tab.Name) == "" {
			return fmt.Errorf("custom tab names must not be empty")
		}
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.CustomTabs = tabs
	return a.save()
}

// SetActionSets replaces the sidebar quick-action sets.
func (a *GuiApp) SetActionSets(sets []config.ActionSet) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.ActionSets = sets
	return a.save()
}

// SetQuickCycleModes replaces the Alt+M quick-cycle mode list.
func (a *GuiApp) SetQuickCycleModes(modes []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	a.cfg().UI.QuickCycleModes = modes
	return a.save()
}

// SetHighPriority replaces the high-priority command list and applies it to
// the engine queue.
func (a *GuiApp) SetHighPriority(cmds []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := append([]string(nil), a.cfg().Commands.HighPriority...)
	a.cfg().Commands.HighPriority = cmds
	a.client().Engine.SetHighPriority(cmds)
	if err := a.save(); err != nil {
		a.client().Engine.SetHighPriority(old)
		return err
	}
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

// SetNotifications replaces desktop notification settings and applies them.
func (a *GuiApp) SetNotifications(cfg config.DesktopNotificationsConfig) error {
	if cfg.HealthBelow.Threshold < 0 || cfg.HealthBelow.Threshold > 100 ||
		cfg.FatigueBelow.Threshold < 0 || cfg.FatigueBelow.Threshold > 100 {
		return fmt.Errorf("notification thresholds must be between 0 and 100")
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	old := a.cfg().Notifications.Desktop
	a.cfg().Notifications.Desktop = cfg
	a.deps.DesktopNotify.UpdateConfig(cfg)
	if err := a.save(); err != nil {
		a.deps.DesktopNotify.UpdateConfig(old)
		return err
	}
	return nil
}

// SetScriptDirs replaces the Lua script directories and reloads modes from the
// new locations.
func (a *GuiApp) SetScriptDirs(dirs []string) error {
	a.configMu.Lock()
	defer a.configMu.Unlock()
	expanded := make([]string, 0, len(dirs))
	for _, d := range dirs {
		expanded = append(expanded, expandPath(d))
	}
	oldDirs := append([]string(nil), a.cfg().Scripts...)
	oldExpanded := append([]string(nil), a.deps.ScriptDirs...)
	if err := a.client().Engine.UpdateScriptDirs(expanded); err != nil {
		return err
	}
	a.cfg().Scripts = append([]string(nil), dirs...)
	a.deps.ScriptDirs = expanded
	if err := a.save(); err != nil {
		a.cfg().Scripts = oldDirs
		a.deps.ScriptDirs = oldExpanded
		_ = a.client().Engine.UpdateScriptDirs(oldExpanded)
		return err
	}
	return nil
}
