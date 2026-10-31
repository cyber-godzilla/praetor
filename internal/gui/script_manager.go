package gui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/engine"
	"github.com/cyber-godzilla/praetor/internal/scriptmanager"
)

// ScriptManagerState is the complete editor model shared by the native and
// browser shells. Local directories stay user-owned; managed sources live in
// Praetor-owned storage and enter the runtime only when explicitly enabled.
type ScriptManagerState struct {
	LocalDirectories   []LocalScriptDirectoryView `json:"localDirectories"`
	Managed            []ManagedScriptSourceView  `json:"managed"`
	SuggestedURL       string                     `json:"suggestedURL"`
	SuggestedInstalled bool                       `json:"suggestedInstalled"`
}

type LocalScriptDirectoryView struct {
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

type ManagedScriptSourceView struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`
	URL              string                     `json:"url"`
	Enabled          bool                       `json:"enabled"`
	Trust            string                     `json:"trust"`
	CheckOnStartup   bool                       `json:"checkOnStartup"`
	Tracking         string                     `json:"tracking"`
	Ref              string                     `json:"ref"`
	InstalledCommit  string                     `json:"installedCommit"`
	InstalledVersion string                     `json:"installedVersion"`
	Status           scriptmanager.UpdateStatus `json:"status"`
	CanEnable        bool                       `json:"canEnable"`
}

func (a *GuiApp) GetScriptManager() ScriptManagerState {
	a.configMu.RLock()
	dirs := a.effectiveLocalScriptDirsLocked()
	local := make([]LocalScriptDirectoryView, 0, len(dirs))
	for _, dir := range dirs {
		local = append(local, LocalScriptDirectoryView{Path: dir, Enabled: !a.localScriptDisabledLocked(dir)})
	}
	sources := append([]config.ManagedScriptSourceConfig(nil), a.cfg().ManagedScripts...)
	a.configMu.RUnlock()
	a.scriptStatusMu.RLock()
	defer a.scriptStatusMu.RUnlock()
	views := make([]ManagedScriptSourceView, 0, len(sources))
	suggestedInstalled := false
	for _, source := range sources {
		if sameRepositoryURL(source.URL, scriptmanager.SuggestedRepositoryURL) {
			suggestedInstalled = true
		}
		views = append(views, managedScriptView(source, a.scriptStatuses[source.ID]))
	}
	return ScriptManagerState{
		LocalDirectories: local, Managed: views,
		SuggestedURL: scriptmanager.SuggestedRepositoryURL, SuggestedInstalled: suggestedInstalled,
	}
}

// SetLocalScriptEnabled reloads the script graph with one user-owned directory
// included or excluded. The path remains configured so it can be re-enabled
// without finding or typing it again.
func (a *GuiApp) SetLocalScriptEnabled(path string, enabled, stopAutomation bool) error {
	if a.client() == nil || a.client().Engine == nil {
		return errors.New("script runtime is unavailable")
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	activeMode := a.client().Engine.CurrentMode()
	if activeMode != "" && activeMode != "disable" {
		if !stopAutomation {
			return fmt.Errorf("Lua mode %q is active; stop automation before changing script sources", activeMode)
		}
		a.client().Engine.SetMode("disable", nil)
	}

	a.configMu.Lock()
	found := false
	for _, configured := range a.effectiveLocalScriptDirsLocked() {
		if localScriptDirectoryKey(configured) == localScriptDirectoryKey(path) {
			path = configured
			found = true
			break
		}
	}
	if !found {
		a.configMu.Unlock()
		return errors.New("local script directory not found")
	}
	previousDisabled := append([]string(nil), a.cfg().DisabledScripts...)
	previousRuntime := a.client().Engine.ScriptSources()
	a.setLocalScriptDisabledLocked(path, !enabled)
	nextRuntime := a.scriptGraphSourcesLocked()
	a.configMu.Unlock()

	if err := a.client().Engine.UpdateScriptSources(nextRuntime); err != nil {
		a.configMu.Lock()
		a.cfg().DisabledScripts = previousDisabled
		a.configMu.Unlock()
		return err
	}
	a.configMu.Lock()
	if err := a.save(); err != nil {
		a.cfg().DisabledScripts = previousDisabled
		a.configMu.Unlock()
		_ = a.client().Engine.UpdateScriptSources(previousRuntime)
		return err
	}
	a.configMu.Unlock()
	return nil
}

func managedScriptView(source config.ManagedScriptSourceConfig, status scriptmanager.UpdateStatus) ManagedScriptSourceView {
	return ManagedScriptSourceView{
		ID: source.ID, Name: source.Name, URL: source.URL, Enabled: source.Enabled,
		Trust: source.Trust, CheckOnStartup: source.CheckOnStartup,
		Tracking: source.Tracking, Ref: source.Ref, InstalledCommit: source.InstalledCommit,
		InstalledVersion: source.InstalledVersion, Status: status,
		CanEnable: true,
	}
}

func (a *GuiApp) InstallManagedScript(rawURL string) (config.ManagedScriptSourceConfig, error) {
	if a.deps.ScriptManager == nil {
		return config.ManagedScriptSourceConfig{}, errors.New("script manager is unavailable")
	}
	canonical, err := scriptmanager.ValidateRepositoryURL(rawURL)
	if err != nil {
		return config.ManagedScriptSourceConfig{}, err
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.configMu.RLock()
	for _, source := range a.cfg().ManagedScripts {
		if sameRepositoryURL(source.URL, canonical) {
			a.configMu.RUnlock()
			return config.ManagedScriptSourceConfig{}, errors.New("that repository is already installed")
		}
	}
	a.configMu.RUnlock()
	suggested := sameRepositoryURL(canonical, scriptmanager.SuggestedRepositoryURL)
	source, err := a.deps.ScriptManager.Install(context.Background(), scriptmanager.InstallRequest{
		URL: canonical, Suggested: suggested, CheckOnStartup: suggested,
	})
	if err != nil {
		return config.ManagedScriptSourceConfig{}, err
	}
	a.configMu.Lock()
	a.cfg().ManagedScripts = append(a.cfg().ManagedScripts, source)
	err = a.save()
	a.configMu.Unlock()
	if err != nil {
		_ = a.deps.ScriptManager.Remove(source)
		return config.ManagedScriptSourceConfig{}, err
	}
	return source, nil
}

func (a *GuiApp) CheckManagedScript(id string) (scriptmanager.UpdateStatus, error) {
	_, status, err := a.checkAndUpdateManagedScript(id)
	return status, err
}

// checkAndUpdateManagedScript implements the manual one-click update flow. A
// background startup check deliberately calls scriptmanager.Manager.Check
// directly and remains notification-only.
func (a *GuiApp) checkAndUpdateManagedScript(id string) (config.ManagedScriptSourceConfig, scriptmanager.UpdateStatus, error) {
	if a.deps.ScriptManager == nil {
		return config.ManagedScriptSourceConfig{}, scriptmanager.UpdateStatus{}, errors.New("script manager is unavailable")
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	source, err := a.managedScriptSource(id)
	if err != nil {
		return config.ManagedScriptSourceConfig{}, scriptmanager.UpdateStatus{}, err
	}
	status, err := a.deps.ScriptManager.Check(context.Background(), source)
	if err != nil {
		status.Error = err.Error()
		a.setManagedScriptStatus(id, status)
		return source, status, err
	}
	if !status.Available {
		status.Error = ""
		a.setManagedScriptStatus(id, status)
		return source, status, nil
	}
	if a.client() == nil || a.client().Engine == nil {
		err = errors.New("script runtime is unavailable")
		status.Error = err.Error()
		a.setManagedScriptStatus(id, status)
		return source, status, err
	}

	// Hold the producer lock from the active-mode preflight through the runtime
	// rebuild so a new Lua mode cannot start in the middle of the update.
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	activeMode := a.client().Engine.CurrentMode()
	if activeMode != "" && activeMode != "disable" {
		err = fmt.Errorf("Lua mode %q is active; stop automation before updating script sources", activeMode)
		status.Error = err.Error()
		a.setManagedScriptStatus(id, status)
		return source, status, err
	}

	previousRuntime := a.client().Engine.ScriptSources()
	if source.Enabled {
		a.configMu.RLock()
		disabledRuntime := a.scriptGraphSourcesExcludingLocked(id)
		a.configMu.RUnlock()
		if err = a.client().Engine.UpdateScriptSources(disabledRuntime); err != nil {
			status.Error = err.Error()
			a.setManagedScriptStatus(id, status)
			return source, status, err
		}
	}

	// Manager.Update requires a disabled source because it has no runtime
	// authority. The persisted source remains enabled; the callback reloads the
	// new checkout before committing its revision metadata.
	updateSource := source
	updateSource.Enabled = false
	committed := false
	committedSource := source
	updated, appliedStatus, updateErr := a.deps.ScriptManager.Update(context.Background(), updateSource, func(next config.ManagedScriptSourceConfig) error {
		next.Enabled = source.Enabled
		if source.Enabled {
			a.configMu.RLock()
			nextRuntime := a.scriptGraphSourcesLocked()
			a.configMu.RUnlock()
			if err := a.client().Engine.UpdateScriptSources(nextRuntime); err != nil {
				return fmt.Errorf("loading updated scripts: %w", err)
			}
		}

		a.configMu.Lock()
		index := managedScriptIndex(a.cfg().ManagedScripts, id)
		if index < 0 {
			a.configMu.Unlock()
			return errors.New("managed script source no longer exists")
		}
		previousConfig := a.cfg().ManagedScripts[index]
		a.cfg().ManagedScripts[index] = next
		if err := a.save(); err != nil {
			a.cfg().ManagedScripts[index] = previousConfig
			a.configMu.Unlock()
			return err
		}
		a.configMu.Unlock()
		committed = true
		committedSource = next
		return nil
	})

	if !committed && source.Enabled {
		if restoreErr := a.client().Engine.UpdateScriptSources(previousRuntime); restoreErr != nil {
			if updateErr == nil {
				updateErr = fmt.Errorf("restoring scripts after update check: %w", restoreErr)
			} else {
				updateErr = fmt.Errorf("%v; restoring scripts: %w", updateErr, restoreErr)
			}
		}
	}
	if updateErr != nil {
		if committed {
			updated = committedSource
			appliedStatus.Updated = true
			appliedStatus.Available = false
		}
		appliedStatus.Error = updateErr.Error()
		a.setManagedScriptStatus(id, appliedStatus)
		return updated, appliedStatus, updateErr
	}
	if !committed {
		appliedStatus.Error = ""
		a.setManagedScriptStatus(id, appliedStatus)
		return source, appliedStatus, nil
	}

	updated = committedSource
	appliedStatus.Available = false
	appliedStatus.Updated = true
	appliedStatus.Error = ""
	a.setManagedScriptStatus(id, appliedStatus)
	return updated, appliedStatus, nil
}

func (a *GuiApp) UpdateManagedScript(id string) (config.ManagedScriptSourceConfig, error) {
	source, _, err := a.checkAndUpdateManagedScript(id)
	return source, err
}

func (a *GuiApp) RemoveManagedScript(id string) error {
	if a.deps.ScriptManager == nil {
		return errors.New("script manager is unavailable")
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.configMu.Lock()
	index := managedScriptIndex(a.cfg().ManagedScripts, id)
	if index < 0 {
		a.configMu.Unlock()
		return errors.New("managed script source not found")
	}
	source := a.cfg().ManagedScripts[index]
	if source.Enabled {
		a.configMu.Unlock()
		return errors.New("disable this source before removing it")
	}
	old := append([]config.ManagedScriptSourceConfig(nil), a.cfg().ManagedScripts...)
	a.cfg().ManagedScripts = append(a.cfg().ManagedScripts[:index:index], a.cfg().ManagedScripts[index+1:]...)
	if err := a.save(); err != nil {
		a.configMu.Unlock()
		return err
	}
	a.configMu.Unlock()
	if err := a.deps.ScriptManager.Remove(source); err != nil {
		a.configMu.Lock()
		a.cfg().ManagedScripts = old
		saveErr := a.save()
		a.configMu.Unlock()
		if saveErr != nil {
			return fmt.Errorf("removing scripts: %v; restoring configuration: %w", err, saveErr)
		}
		return err
	}
	a.scriptStatusMu.Lock()
	delete(a.scriptStatuses, id)
	a.scriptStatusMu.Unlock()
	return nil
}

func (a *GuiApp) SetManagedScriptStartupCheck(id string, enabled bool) error {
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.configMu.Lock()
	defer a.configMu.Unlock()
	index := managedScriptIndex(a.cfg().ManagedScripts, id)
	if index < 0 {
		return errors.New("managed script source not found")
	}
	a.cfg().ManagedScripts[index].CheckOnStartup = enabled
	return a.save()
}

func (a *GuiApp) SetManagedScriptEnabled(id string, enabled, stopAutomation bool) error {
	return a.changeManagedScriptRuntime(id, stopAutomation, func(source *config.ManagedScriptSourceConfig) error {
		source.Enabled = enabled
		return nil
	})
}

func (a *GuiApp) SetManagedScriptTrust(id, trust string, stopAutomation bool) error {
	if trust != config.ScriptTrustRestricted && trust != config.ScriptTrustTrusted {
		return errors.New("script trust must be restricted or trusted")
	}
	return a.changeManagedScriptRuntime(id, stopAutomation, func(source *config.ManagedScriptSourceConfig) error {
		source.Trust = trust
		return nil
	})
}

func (a *GuiApp) changeManagedScriptRuntime(id string, stopAutomation bool, mutate func(*config.ManagedScriptSourceConfig) error) error {
	if a.client() == nil || a.client().Engine == nil || a.deps.ScriptManager == nil {
		return errors.New("script runtime is unavailable")
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	activeMode := a.client().Engine.CurrentMode()
	if activeMode != "" && activeMode != "disable" {
		if !stopAutomation {
			return fmt.Errorf("Lua mode %q is active; stop automation before changing script sources", activeMode)
		}
		a.client().Engine.SetMode("disable", nil)
	}
	a.configMu.Lock()
	index := managedScriptIndex(a.cfg().ManagedScripts, id)
	if index < 0 {
		a.configMu.Unlock()
		return errors.New("managed script source not found")
	}
	previousConfig := a.cfg().ManagedScripts[index]
	previousRuntime := a.client().Engine.ScriptSources()
	if err := mutate(&a.cfg().ManagedScripts[index]); err != nil {
		a.configMu.Unlock()
		return err
	}
	nextRuntime := a.scriptGraphSourcesLocked()
	a.configMu.Unlock()

	// Loading a script executes its top-level chunk. Do not hold configMu here:
	// a legitimate variables.set() in that chunk synchronously persists through
	// the GUI and needs the same lock.
	if err := a.client().Engine.UpdateScriptSources(nextRuntime); err != nil {
		a.configMu.Lock()
		index = managedScriptIndex(a.cfg().ManagedScripts, id)
		if index >= 0 {
			a.cfg().ManagedScripts[index] = previousConfig
		}
		a.configMu.Unlock()
		return err
	}
	a.configMu.Lock()
	if err := a.save(); err != nil {
		index = managedScriptIndex(a.cfg().ManagedScripts, id)
		if index >= 0 {
			a.cfg().ManagedScripts[index] = previousConfig
		}
		a.configMu.Unlock()
		_ = a.client().Engine.UpdateScriptSources(previousRuntime)
		return err
	}
	a.configMu.Unlock()
	return nil
}

func (a *GuiApp) ReorderManagedScripts(ids []string) error {
	if a.client() == nil || a.client().Engine == nil || a.deps.ScriptManager == nil {
		return errors.New("script runtime is unavailable")
	}
	a.scriptManagerMu.Lock()
	defer a.scriptManagerMu.Unlock()
	a.configMu.Lock()
	if len(ids) != len(a.cfg().ManagedScripts) {
		a.configMu.Unlock()
		return errors.New("managed script order must include every source exactly once")
	}
	byID := make(map[string]config.ManagedScriptSourceConfig, len(ids))
	for _, source := range a.cfg().ManagedScripts {
		byID[source.ID] = source
	}
	reordered := make([]config.ManagedScriptSourceConfig, 0, len(ids))
	for _, id := range ids {
		source, ok := byID[id]
		if !ok {
			a.configMu.Unlock()
			return errors.New("managed script order contains an unknown or duplicate source")
		}
		delete(byID, id)
		reordered = append(reordered, source)
	}
	previous := append([]config.ManagedScriptSourceConfig(nil), a.cfg().ManagedScripts...)
	previousRuntime := a.client().Engine.ScriptSources()
	a.cfg().ManagedScripts = reordered
	nextRuntime := a.scriptGraphSourcesLocked()
	a.configMu.Unlock()
	if err := a.client().Engine.UpdateScriptSources(nextRuntime); err != nil {
		a.configMu.Lock()
		a.cfg().ManagedScripts = previous
		a.configMu.Unlock()
		return err
	}
	a.configMu.Lock()
	if err := a.save(); err != nil {
		a.cfg().ManagedScripts = previous
		a.configMu.Unlock()
		_ = a.client().Engine.UpdateScriptSources(previousRuntime)
		return err
	}
	a.configMu.Unlock()
	return nil
}

func (a *GuiApp) managedScriptSource(id string) (config.ManagedScriptSourceConfig, error) {
	a.configMu.RLock()
	defer a.configMu.RUnlock()
	if index := managedScriptIndex(a.cfg().ManagedScripts, id); index >= 0 {
		return a.cfg().ManagedScripts[index], nil
	}
	return config.ManagedScriptSourceConfig{}, errors.New("managed script source not found")
}

func managedScriptIndex(sources []config.ManagedScriptSourceConfig, id string) int {
	for index := range sources {
		if sources[index].ID == id {
			return index
		}
	}
	return -1
}

func sameRepositoryURL(left, right string) bool {
	left, leftErr := scriptmanager.ValidateRepositoryURL(left)
	right, rightErr := scriptmanager.ValidateRepositoryURL(right)
	return leftErr == nil && rightErr == nil && left == right
}

func (a *GuiApp) scriptGraphSourcesLocked() []engine.LuaSource {
	return a.scriptGraphSourcesExcludingLocked("")
}

func (a *GuiApp) scriptGraphSourcesExcludingLocked(excludedID string) []engine.LuaSource {
	sources := make([]engine.LuaSource, 0, len(a.deps.ScriptDirs)+len(a.cfg().ManagedScripts))
	for _, dir := range a.deps.ScriptDirs {
		if !a.localScriptDisabledLocked(dir) {
			sources = append(sources, engine.LuaSource{Path: dir, Trust: engine.LuaTrustTrusted})
		}
	}
	for _, source := range a.cfg().ManagedScripts {
		if source.Enabled && source.ID != excludedID {
			sources = append(sources, engine.LuaSource{Path: a.deps.ScriptManager.Path(source.ID), Trust: source.Trust})
		}
	}
	return sources
}

func (a *GuiApp) effectiveLocalScriptDirsLocked() []string {
	if len(a.cfg().Scripts) > 0 {
		return append([]string(nil), a.cfg().Scripts...)
	}
	return append([]string(nil), a.deps.ScriptDirs...)
}

func localScriptDirectoryKey(path string) string {
	return filepath.Clean(expandPath(path))
}

func (a *GuiApp) localScriptDisabledLocked(path string) bool {
	key := localScriptDirectoryKey(path)
	for _, disabled := range a.cfg().DisabledScripts {
		if localScriptDirectoryKey(disabled) == key {
			return true
		}
	}
	return false
}

func (a *GuiApp) setLocalScriptDisabledLocked(path string, disabled bool) {
	key := localScriptDirectoryKey(path)
	next := make([]string, 0, len(a.cfg().DisabledScripts)+1)
	for _, candidate := range a.cfg().DisabledScripts {
		if localScriptDirectoryKey(candidate) != key {
			next = append(next, candidate)
		}
	}
	if disabled {
		next = append(next, path)
	}
	a.cfg().DisabledScripts = next
}

func (a *GuiApp) setManagedScriptStatus(id string, status scriptmanager.UpdateStatus) {
	a.scriptStatusMu.Lock()
	a.scriptStatuses[id] = status
	a.scriptStatusMu.Unlock()
}

func (a *GuiApp) checkStartupScriptSources(ctx context.Context) {
	if a.deps.ScriptManager == nil {
		return
	}
	a.configMu.RLock()
	sources := append([]config.ManagedScriptSourceConfig(nil), a.cfg().ManagedScripts...)
	a.configMu.RUnlock()
	for _, source := range sources {
		if !source.CheckOnStartup {
			continue
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		status, err := a.deps.ScriptManager.Check(ctx, source)
		if err != nil {
			status.Error = err.Error()
			log.Printf("[SCRIPTS] update check for %s failed: %v", source.Name, err)
		}
		a.setManagedScriptStatus(source.ID, status)
		if status.Available {
			version := status.Version
			if version == "" {
				version = status.Ref
			}
			a.notify("Script update available", fmt.Sprintf("%s %s is available.", source.Name, version))
		}
	}
}
