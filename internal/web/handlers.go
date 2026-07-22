package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	appgui "github.com/cyber-godzilla/praetor/internal/gui"
	"github.com/cyber-godzilla/praetor/internal/session"
)

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Store    bool   `json:"store"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Username) == "" || req.Password == "" || len(req.Username) > 256 || len(req.Password) > 4096 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Username and password are required.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.conn != "disconnected" {
		s.writeError(w, http.StatusConflict, "session_busy", "A shared game session is already active or connecting.")
		return
	}
	s.conn = "connecting"
	result, err := s.app.ConnectNew(req.Username, req.Password, req.Store)
	req.Password = ""
	if err != nil {
		s.conn = "disconnected"
		s.log.Printf("web game connect failed: %v", err)
		s.writeError(w, http.StatusBadGateway, "game_connect_failed", "Unable to connect to the shared game session.")
		return
	}
	s.conn = "connected"
	if result.AccountState != nil {
		s.broadcastAccountStateLocked(*result.AccountState)
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleConnectStored(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Username) == "" || len(req.Username) > 256 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "An account name is required.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.conn != "disconnected" {
		s.writeError(w, http.StatusConflict, "session_busy", "A shared game session is already active or connecting.")
		return
	}
	s.conn = "connecting"
	if err := s.app.ConnectStored(req.Username); err != nil {
		s.conn = "disconnected"
		s.log.Printf("web stored-account connect failed: %v", err)
		var credentialErr *appgui.CredentialStoreError
		if errors.As(err, &credentialErr) {
			state := s.app.ListAccounts()
			s.broadcastAccountStateLocked(state)
			if errors.Is(err, session.ErrNoCredentials) {
				s.writeError(w, http.StatusNotFound, "stored_account_not_found", "Stored credentials were not found for this account.")
				return
			}
			s.writeError(w, http.StatusServiceUnavailable, "credential_store_unavailable", state.CredentialStore.Message)
			return
		}
		s.writeError(w, http.StatusBadGateway, "game_connect_failed", "Unable to connect with the stored account.")
		return
	}
	s.conn = "connected"
	s.writeJSON(w, http.StatusOK, appgui.ConnectResult{Connected: true})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if s.conn == "disconnected" {
		s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	s.conn = "disconnecting"
	s.app.Disconnect()
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Input string `json:"input"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid command request.")
		return
	}
	if len(req.Input) > maxCommandBody {
		s.writeError(w, http.StatusRequestEntityTooLarge, "command_too_large", "Command is too large.")
		return
	}
	s.opMu.Lock()
	if s.conn != "connected" {
		s.opMu.Unlock()
		s.writeError(w, http.StatusConflict, "session_not_connected", "The shared game session is not connected.")
		return
	}
	s.app.Send(req.Input)
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleAccounts(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	state := s.app.ListAccounts()
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleSaveAccount(w http.ResponseWriter, r *http.Request, _ string) {
	name, err := url.PathUnescape(r.PathValue("name"))
	if err != nil || strings.TrimSpace(name) == "" || len(name) > 256 {
		s.writeError(w, http.StatusBadRequest, "invalid_account", "Invalid account name.")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Password == "" || len(req.Password) > 4096 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "A password is required.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	err = s.app.SaveAccount(name, req.Password)
	req.Password = ""
	if err != nil {
		s.log.Printf("web account save failed: %v", err)
		state := s.app.ListAccounts()
		s.broadcastAccountStateLocked(state)
		s.writeError(w, http.StatusServiceUnavailable, "credential_store_unavailable", state.CredentialStore.Message)
		return
	}
	s.broadcastAccountsLocked()
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveAccount(w http.ResponseWriter, r *http.Request, _ string) {
	name, err := url.PathUnescape(r.PathValue("name"))
	if err != nil || strings.TrimSpace(name) == "" || len(name) > 256 {
		s.writeError(w, http.StatusBadRequest, "invalid_account", "Invalid account name.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.app.RemoveAccount(name); err != nil {
		s.log.Printf("web account removal failed: %v", err)
		state := s.app.ListAccounts()
		s.broadcastAccountStateLocked(state)
		s.writeError(w, http.StatusServiceUnavailable, "credential_store_unavailable", state.CredentialStore.Message)
		return
	}
	s.broadcastAccountsLocked()
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleModes(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	names := s.app.ModeNames()
	current := s.app.CurrentMode()
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusOK, map[string]any{"modeNames": names, "currentMode": current})
}

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Name string   `json:"name"`
		Args []string `json:"args"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid mode request.")
		return
	}
	if len(req.Name) > 256 || len(req.Args) > 64 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid mode request.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.app.SetMode(req.Name, req.Args); err != nil {
		s.writeError(w, http.StatusBadRequest, "mode_failed", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleReloadScripts(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.app.ReloadScripts(); err != nil {
		s.hub.BroadcastState(Envelope{Type: "operation", Result: &OperationResult{Operation: "reloadScripts", Message: err.Error()}})
		s.writeError(w, http.StatusInternalServerError, "script_reload_failed", err.Error())
		return
	}
	names := s.app.ModeNames()
	s.hub.BroadcastState(Envelope{Type: "modes", ModeNames: names, Result: &OperationResult{Operation: "reloadScripts", OK: true}})
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true, "modeNames": names})
}

func (s *Server) handleRefreshGraphics(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	s.app.RefreshGraphics()
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (s *Server) handleSetting(w http.ResponseWriter, r *http.Request, _ string) {
	operation := r.PathValue("operation")
	var req settingRequest
	if err := decodeJSON(r, &req); err != nil || req.ExpectedRevision == nil || len(req.Value) == 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid setting value.")
		return
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()
	if *req.ExpectedRevision != s.revision {
		s.writeError(w, http.StatusConflict, "revision_conflict", "Settings changed in another browser; reload and try again.")
		return
	}
	if err := s.applySetting(operation, req.Value); err != nil {
		s.writeError(w, http.StatusBadRequest, "setting_failed", err.Error())
		return
	}
	s.revision++
	configJSON := cloneJSON(s.app.GetConfig())
	s.hub.BroadcastState(Envelope{Type: "config", Config: configJSON, Revision: s.revision})
	if operation == "script-directories" {
		names := s.app.ModeNames()
		s.hub.BroadcastState(Envelope{Type: "modes", ModeNames: names, Result: &OperationResult{
			Operation: "scriptDirectories", OK: true,
		}})
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"config": json.RawMessage(configJSON), "revision": s.revision})
}

func (s *Server) applySetting(operation string, raw json.RawMessage) error {
	switch operation {
	case "echo-typed":
		return settingValue(raw, s.app.SetEchoTyped)
	case "echo-script":
		return settingValue(raw, s.app.SetEchoScript)
	case "color-words":
		return settingValue(raw, s.app.SetColorWords)
	case "hide-ips":
		return settingValue(raw, s.app.SetHideIPs)
	case "input-spellcheck":
		return settingValue(raw, s.app.SetInputSpellcheck)
	case "update-check":
		return settingValue(raw, s.app.SetUpdateCheck)
	case "mobile-show-toolbar":
		return settingValue(raw, s.app.SetMobileShowToolbar)
	case "mobile-show-tab-bar":
		return settingValue(raw, s.app.SetMobileShowTabBar)
	case "mobile-hide-navigation-on-input":
		return settingValue(raw, s.app.SetMobileHideNavigationOnInput)
	case "mobile-lowercase-first-letter":
		return settingValue(raw, s.app.SetMobileLowercaseFirstLetter)
	case "mobile-output-font-size":
		return settingValue(raw, s.app.SetMobileOutputFontSize)
	case "session-logging":
		return settingValue(raw, s.app.SetSessionLogging)
	case "log-path":
		return settingValue(raw, s.app.SetLogPath)
	case "display-mode":
		return settingValue(raw, s.app.SetDisplayMode)
	case "numpad-navigation":
		return settingValue(raw, s.app.SetNumpadNavigation)
	case "minimap-scale":
		return settingValue(raw, s.app.SetMinimapScale)
	case "compass-scale":
		return settingValue(raw, s.app.SetCompassScale)
	case "output-font-size":
		return settingValue(raw, s.app.SetOutputFontSize)
	case "crt-effects":
		value, err := decodeValue[struct {
			Scanlines bool `json:"scanlines"`
			Roll      bool `json:"roll"`
			Bloom     bool `json:"bloom"`
		}](raw)
		if err != nil {
			return err
		}
		return s.app.SetCRTEffects(value.Scanlines, value.Roll, value.Bloom)
	case "highlights":
		return settingValue[[]config.HighlightConfig](raw, s.app.SetHighlights)
	case "custom-tabs":
		return settingValue[[]config.CustomTabConfig](raw, s.app.SetCustomTabs)
	case "action-sets":
		return settingValue[[]config.ActionSet](raw, s.app.SetActionSets)
	case "quick-cycle-modes":
		return settingValue[[]string](raw, s.app.SetQuickCycleModes)
	case "high-priority":
		return settingValue[[]string](raw, s.app.SetHighPriority)
	case "ignore-ooc":
		return settingValue[[]string](raw, s.app.SetIgnoreOOC)
	case "ignore-think":
		return settingValue[[]string](raw, s.app.SetIgnoreThink)
	case "notifications":
		return settingValue[config.DesktopNotificationsConfig](raw, s.app.SetNotifications)
	case "script-directories":
		return settingValue[[]string](raw, s.app.SetScriptDirs)
	default:
		return fmt.Errorf("unknown setting %q", operation)
	}
}

func (s *Server) handleGetKudos(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	value := s.app.GetKudos()
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) handleSetKudos(w http.ResponseWriter, r *http.Request, _ string) {
	var req settingRequest
	if err := decodeJSON(r, &req); err != nil || req.ExpectedRevision == nil || len(req.Value) == 0 {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid Kudos data.")
		return
	}
	value, err := decodeValue[config.KudosConfig](req.Value)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid Kudos data.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if *req.ExpectedRevision != s.revision {
		s.writeError(w, http.StatusConflict, "revision_conflict", "Settings changed in another browser; reload and try again.")
		return
	}
	if err := s.app.SetKudos(value); err != nil {
		s.writeError(w, http.StatusInternalServerError, "kudos_save_failed", err.Error())
		return
	}
	configJSON := s.commitConfigLocked()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "config": json.RawMessage(configJSON), "revision": s.revision,
	})
}

func (s *Server) handleAddKudosFavorite(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid favorite.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	added, err := s.app.AddKudosFavorite(req.Name)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "kudos_save_failed", err.Error())
		return
	}
	if added {
		s.commitConfigLocked()
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"added": added, "config": s.app.GetConfig(), "revision": s.revision,
	})
}

func (s *Server) handleAddKudosQueue(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid queue entry.")
		return
	}
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if err := s.app.AddKudosQueue(req.Name, req.Message); err != nil {
		s.writeError(w, http.StatusInternalServerError, "kudos_save_failed", err.Error())
		return
	}
	configJSON := s.commitConfigLocked()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "config": json.RawMessage(configJSON), "revision": s.revision,
	})
}

func (s *Server) handlePersistentData(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	value := s.app.GetPersistentData()
	s.opMu.Unlock()
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) handlePersistentExport(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid export selection.")
		return
	}
	s.opMu.Lock()
	data, err := s.app.PersistentDataJSON(req.Keys)
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	filename := "persistent_" + time.Now().Format("2006-01-02_150405") + ".json"
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handlePersistentClear(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid clear selection.")
		return
	}
	s.opMu.Lock()
	err := s.app.ClearPersistentData(req.Keys)
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "persistent_clear_failed", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListNotes(w http.ResponseWriter, _ *http.Request, _ string) {
	s.opMu.Lock()
	value, err := s.app.ListNotes()
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "notes_read_failed", "Unable to list notes.")
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request, _ string) {
	title, err := url.PathUnescape(r.PathValue("title"))
	if err != nil || strings.TrimSpace(title) == "" || len(title) > 512 {
		s.writeError(w, http.StatusBadRequest, "invalid_note", "Invalid note title.")
		return
	}
	s.opMu.Lock()
	value, err := s.app.GetNote(title)
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusNotFound, "note_not_found", "Note not found.")
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) handleSaveNote(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		OriginalTitle string `json:"originalTitle"`
		Title         string `json:"title"`
		Body          string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Title) == "" || len(req.Title) > 512 {
		s.writeError(w, http.StatusBadRequest, "invalid_note", "A valid note title is required.")
		return
	}
	s.opMu.Lock()
	err := s.app.SaveNote(req.OriginalTitle, req.Title, req.Body)
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "note_save_failed", err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request, _ string) {
	title, err := url.PathUnescape(r.PathValue("title"))
	if err != nil || strings.TrimSpace(title) == "" || len(title) > 512 {
		s.writeError(w, http.StatusBadRequest, "invalid_note", "Invalid note title.")
		return
	}
	s.opMu.Lock()
	err = s.app.DeleteNote(title)
	s.opMu.Unlock()
	if err != nil {
		s.writeError(w, http.StatusNotFound, "note_not_found", "Note not found.")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleWiki(w http.ResponseWriter, _ *http.Request, _ string) {
	s.writeJSON(w, http.StatusOK, s.app.GetWikiSections())
}

func (s *Server) handleMaps(w http.ResponseWriter, _ *http.Request, _ string) {
	s.writeJSON(w, http.StatusOK, s.app.GetMapSections())
}

func (s *Server) handleRankBonus(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Mode     int `json:"mode"`
		Basics   int `json:"basics"`
		Subskill int `json:"subskill"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid calculation input.")
		return
	}
	s.writeJSON(w, http.StatusOK, s.app.CalcRankBonus(req.Mode, req.Basics, req.Subskill))
}

func (s *Server) handleTrainCost(w http.ResponseWriter, r *http.Request, _ string) {
	var req struct {
		Current     int  `json:"current"`
		Desired     int  `json:"desired"`
		Slot        int  `json:"slot"`
		Difficulty  int  `json:"difficulty"`
		SelfTrained bool `json:"selfTrained"`
		SelfTaught  bool `json:"selfTaught"`
		Healing     bool `json:"healing"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid calculation input.")
		return
	}
	value := s.app.CalcTrainCost(req.Current, req.Desired, req.Slot, req.Difficulty, req.SelfTrained, req.SelfTaught, req.Healing)
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) commitConfigLocked() json.RawMessage {
	s.revision++
	configJSON := cloneJSON(s.app.GetConfig())
	s.hub.BroadcastState(Envelope{Type: "config", Config: configJSON, Revision: s.revision})
	return configJSON
}

func (s *Server) broadcastAccountsLocked() {
	s.broadcastAccountStateLocked(s.app.ListAccounts())
}

func (s *Server) broadcastAccountStateLocked(state appgui.AccountState) {
	status := state.CredentialStore
	s.hub.BroadcastState(Envelope{Type: "accounts", Accounts: cloneAccounts(state.Accounts), CredentialStore: &status})
}
