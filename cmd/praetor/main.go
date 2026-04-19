package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/logging"
	"github.com/cyber-godzilla/praetor/internal/session"
	"github.com/cyber-godzilla/praetor/internal/types"
	"github.com/cyber-godzilla/praetor/internal/ui"
)

// Set via ldflags: go build -ldflags "-X main.version=v1.0.0"
var version = "dev"

// wrapper wraps the App model and intercepts messages to wire them to the Client.
type wrapper struct {
	app           ui.App
	gc            *client.Client
	prog          *tea.Program
	cfg           *config.Config
	cfgPath       string
	fromLogin     bool   // true if auth came from login form (not account select)
	lastUsername  string // username from login form, preserves original casing
	lastPassword  string // password from login form, cleared after use
	dataDir       string
	configDir     string
	desktopNotify *client.DesktopNotifier
}

func (w wrapper) Init() tea.Cmd {
	return w.app.Init()
}

func (w wrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.AccountSelectMsg:
		w.fromLogin = false
		// Let the app transition to "authenticating" state
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)

		// Look up stored password and perform HTTP login asynchronously
		username := msg.Username
		loginCmd := func() tea.Msg {
			pass, err := w.gc.Creds.GetAccount(username)
			if err != nil {
				return ui.AuthResultMsg{
					Success: false,
					Error:   "stored credentials not found: " + err.Error(),
				}
			}
			err = w.gc.Login(username, pass)
			if err != nil {
				return ui.AuthResultMsg{
					Success: false,
					Error:   err.Error(),
				}
			}
			return ui.AuthResultMsg{Success: true}
		}
		return w, tea.Batch(cmd, loginCmd)

	case ui.DeleteAccountMsg:
		// Let the app handle the message first
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)

		// Remove account from store and refresh list
		username := msg.Username
		refreshCmd := func() tea.Msg {
			if err := w.gc.Creds.RemoveAccount(username); err != nil {
				log.Printf("failed to remove account %q: %v", username, err)
			}
			accounts, err := w.gc.Creds.ListAccounts()
			if err != nil {
				log.Printf("failed to list accounts after delete: %v", err)
				accounts = nil
			}
			return ui.AccountListUpdatedMsg{Accounts: accounts}
		}
		return w, tea.Batch(cmd, refreshCmd)

	case ui.LoginSubmitMsg:
		w.fromLogin = true
		w.lastUsername = msg.Username
		w.lastPassword = msg.Password
		// Let the app transition to "authenticating" state
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)

		// Perform HTTP login asynchronously
		loginCmd := func() tea.Msg {
			err := w.gc.Login(msg.Username, msg.Password)
			if err != nil {
				return ui.AuthResultMsg{
					Success: false,
					Error:   err.Error(),
				}
			}
			return ui.AuthResultMsg{Success: true}
		}
		return w, tea.Batch(cmd, loginCmd)

	case ui.AuthResultMsg:
		if msg.Success {
			if w.fromLogin {
				// Ask whether to store credentials
				newApp, cmd := w.app.Update(msg)
				w.app = newApp.(ui.App)
				username := w.lastUsername
				password := w.lastPassword
				promptCmd := func() tea.Msg {
					_, err := w.gc.Creds.GetAccount(username)
					return ui.CredentialPromptMsg{
						Username:      username,
						Password:      password,
						AlreadyStored: err == nil,
					}
				}
				return w, tea.Batch(cmd, promptCmd)
			}
			// From account select — credentials already stored, go to game
			newApp, cmd := w.app.Update(msg)
			w.app = newApp.(ui.App)
			go func() {
				if err := w.gc.ConnectWebSocket(); err != nil {
					log.Printf("WebSocket connect error: %v", err)
					return
				}
				w.gc.Run()
			}()
			return w, cmd
		}
		// Auth failed — let the app handle it
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		return w, cmd

	case ui.CredentialStoreMsg:
		if msg.Store {
			if err := w.gc.Creds.SetAccount(msg.Username, msg.Password); err != nil {
				log.Printf("failed to save credentials: %v", err)
			}
		}
		w.lastUsername = "" // clear sensitive data
		w.lastPassword = ""

		// Transition to game
		w.app.SetLoggedIn(true)
		go func() {
			if err := w.gc.ConnectWebSocket(); err != nil {
				log.Printf("WebSocket connect error: %v", err)
				return
			}
			w.gc.Run()
		}()
		return w, nil

	case ui.HelpSearchMsg:
		if msg.Query == "__wiki__" {
			go client.OpenBrowser("https://eternal-city.wikidot.com")
		} else {
			w.gc.SendCommand("?" + msg.Query)
		}
		return w, nil

	case ui.InputSubmitMsg:
		inputCmd := strings.TrimSpace(strings.ToLower(msg.Value))
		// Handle /help locally.
		if inputCmd == "/help" {
			w.app.ShowHelp()
			return w, nil
		}
		// Handle /list locally — list available modes.
		if inputCmd == "/list" {
			modes := w.gc.Engine.ModeNames()
			w.app.ShowModeList(modes)
			return w, nil
		}
		// Handle /mode and /sm locally so we can validate before switching.
		if strings.HasPrefix(inputCmd, "/mode ") || strings.HasPrefix(inputCmd, "/sm ") {
			parts := strings.Fields(msg.Value)
			if len(parts) >= 2 {
				mode := parts[1]
				var args []string
				if len(parts) > 2 {
					args = parts[2:]
				}
				if mode != "disable" && mode != "" && !w.gc.Engine.HasMode(mode) {
					w.app.ShowModeError(mode, w.gc.Engine.ModeNames())
					cur := w.gc.Engine.CurrentMode()
					if cur == "" || cur == "disable" {
						w.gc.Engine.SetMode("disable", nil)
					}
					return w, nil
				}
				w.gc.Engine.SetMode(mode, args)
			}
			return w, nil
		}
		// Route user commands to the client
		w.gc.SendCommand(msg.Value)
		// Still let the app process it (for state tracking)
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		return w, cmd

	case ui.MenuEchoTypedMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		w.gc.Settings.EchoTyped = !w.gc.Settings.EchoTyped
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.EchoTyped = w.gc.Settings.EchoTyped
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuEchoScriptMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		w.gc.Settings.EchoScript = !w.gc.Settings.EchoScript
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.EchoScript = w.gc.Settings.EchoScript
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuColorWordsMsg:
		// Toggle color words and save to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.ColorWords = !w.cfg.UI.ColorWords
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuAutoReconnectMsg:
		// Toggle auto reconnect and save to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.Reconnect.Enabled = !w.cfg.Reconnect.Enabled
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuHideIPsMsg:
		// Toggle IP masking and save to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.HideIPs = !w.cfg.UI.HideIPs
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuGameLogsMsg:
		// Toggle session logging and save to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.Logging.Session.Enabled = !w.cfg.Logging.Session.Enabled
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuLogPathMsg:
		// Update log path and save to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.Logging.Session.Path = msg.Path
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.SidebarToggleMsg:
		// Save sidebar state to config.
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.SidebarOpen = msg.Open
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, nil

	case ui.TabEditorCloseMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.CustomTabs = msg.Tabs
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.HighlightsCloseMsg:
		// Save highlights to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.Highlights = msg.Highlights
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.ModePickerCloseMsg:
		// Save quick-cycle modes to config.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if w.cfg != nil && w.cfgPath != "" {
			w.cfg.UI.QuickCycleModes = msg.Modes
			if err := config.Save(w.cfg, w.cfgPath); err != nil {
				log.Printf("saving config: %v", err)
			}
		}
		return w, cmd

	case ui.MenuReloadScriptsMsg:
		// Reload all Lua scripts
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		var reloadErr error
		if err := w.gc.Engine.ReloadAllModes(); err != nil {
			log.Printf("reload all error: %v", err)
			reloadErr = err
		} else {
			log.Printf("all scripts reloaded")
		}
		// Send confirmation back to UI
		newApp2, cmd2 := w.app.Update(ui.ScriptsReloadedMsg{Error: reloadErr})
		w.app = newApp2.(ui.App)
		return w, tea.Batch(cmd, cmd2)

	case ui.ScriptDirsCloseMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if msg.Changed {
			if w.cfg != nil && w.cfgPath != "" {
				w.cfg.Scripts = msg.Dirs
				if err := config.Save(w.cfg, w.cfgPath); err != nil {
					log.Printf("saving config: %v", err)
				}
			}
			expanded := make([]string, 0, len(msg.Dirs))
			for _, d := range msg.Dirs {
				expanded = append(expanded, expandPath(d))
			}
			if err := w.gc.Engine.UpdateScriptDirs(expanded); err != nil {
				log.Printf("updating script dirs: %v", err)
			}
		}
		return w, cmd

	case ui.PriorityCmdsCloseMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if msg.Changed {
			if w.cfg != nil && w.cfgPath != "" {
				w.cfg.Commands.HighPriority = msg.Cmds
				if err := config.Save(w.cfg, w.cfgPath); err != nil {
					log.Printf("saving config: %v", err)
				}
			}
			w.gc.Engine.SetHighPriority(msg.Cmds)
		}
		return w, cmd

	case ui.NotificationSettingsCloseMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		if msg.Changed {
			if w.cfg != nil && w.cfgPath != "" {
				w.cfg.Notifications.Desktop = msg.Config
				if err := config.Save(w.cfg, w.cfgPath); err != nil {
					log.Printf("saving config: %v", err)
				}
			}
			if w.desktopNotify != nil {
				w.desktopNotify.UpdateConfig(msg.Config)
			}
		}
		return w, cmd

	case ui.MenuQuitMsg:
		return w, tea.Quit

	case ui.SetModeMsg:
		// Alt+M cycle or Alt+I quick-set triggered a mode change.
		if msg.Mode != "disable" && msg.Mode != "" && !w.gc.Engine.HasMode(msg.Mode) {
			w.app.ShowModeError(msg.Mode, w.gc.Engine.ModeNames())
			cur := w.gc.Engine.CurrentMode()
			if cur == "" || cur == "disable" {
				w.gc.Engine.SetMode("disable", nil)
			}
			return w, nil
		}
		w.gc.Engine.SetMode(msg.Mode, msg.Args)
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		return w, cmd

	case ui.MenuQuickCycleMsg:
		// Fetch available mode names from the engine and open the picker.
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)

		modes := w.gc.Engine.ModeNames()
		openCmd := func() tea.Msg {
			return ui.OpenModePickerMsg{AllModes: modes}
		}
		return w, tea.Batch(cmd, openCmd)

	case ui.MenuPersistentDataMsg:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)

		snapshotCmd := func() tea.Msg {
			keys := w.gc.Engine.State().PersistentKeys()
			snap := w.gc.Engine.State().PersistentSnapshot()
			var infos []ui.PersistentKeyInfo
			for _, key := range keys {
				summary := ""
				if val, ok := snap[key]; ok {
					summary = describePersistentValue(val)
				}
				infos = append(infos, ui.PersistentKeyInfo{Key: key, ValueSummary: summary})
			}
			return ui.PersistentDataSnapshotMsg{
				Username: w.lastUsername,
				Keys:     infos,
			}
		}
		return w, tea.Batch(cmd, snapshotCmd)

	case ui.PersistentDataExportMsg:
		snap := w.gc.Engine.State().PersistentSnapshot()
		exportData := make(map[string]interface{})
		for _, key := range msg.Keys {
			if val, ok := snap[key]; ok {
				exportData[key] = val
			}
		}

		exportDir := filepath.Join(w.configDir, "exports")
		os.MkdirAll(exportDir, 0755)
		filename := fmt.Sprintf("persistent_%s.json", time.Now().Format("2006-01-02_150405"))
		exportPath := filepath.Join(exportDir, filename)

		out, _ := json.MarshalIndent(exportData, "", "  ")
		if err := os.WriteFile(exportPath, out, 0644); err != nil {
			log.Printf("export error: %v", err)
		} else {
			log.Printf("exported persistent data to %s", exportPath)
			w.app.SetPersistentDataMessage("Exported to " + exportPath)
		}
		return w, nil

	case ui.PersistentDataClearMsg:
		for _, key := range msg.Keys {
			w.gc.Engine.State().ClearPersistentKey(key)
		}
		if w.gc.Engine.PersistentStore() != nil {
			w.gc.Engine.PersistentStore().Flush()
		}
		// Refresh the screen by re-triggering the snapshot.
		refreshCmd := func() tea.Msg { return ui.MenuPersistentDataMsg{} }
		return w, refreshCmd

	default:
		newApp, cmd := w.app.Update(msg)
		w.app = newApp.(ui.App)
		return w, cmd
	}
}

func (w wrapper) View() string {
	return w.app.View()
}

// xdgPath returns the XDG directory for the given env var, falling back
// to $HOME/<defaultSuffix>. The app subdirectory is appended.
func xdgPath(envVar, defaultSuffix, appName string) string {
	dir := os.Getenv(envVar)
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, defaultSuffix)
	}
	return filepath.Join(dir, appName)
}

// expandPath expands ~ to home dir and $ENV_VAR references in a path.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path)
}

func describePersistentValue(val interface{}) string {
	switch v := val.(type) {
	case map[string]interface{}:
		return fmt.Sprintf("%d entries", len(v))
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return fmt.Sprintf("%g", v)
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debug tab and debug-level logging")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("praetor %s\n", version)
		os.Exit(0)
	}

	// XDG directory compliance.
	configDir := xdgPath("XDG_CONFIG_HOME", ".config", "praetor")
	dataDir := xdgPath("XDG_DATA_HOME", ".local/share", "praetor")
	stateDir := xdgPath("XDG_STATE_HOME", ".local/state", "praetor")
	sessionsDir := filepath.Join(configDir, "logs")

	// Set up structured logging in state dir.
	logLevel := "info"
	if *debugFlag {
		logLevel = "debug"
	}
	appLog, err := logging.New(stateDir, "tec.log", logLevel, 5)
	if err != nil {
		log.Fatalf("opening log file: %v", err)
	}
	defer appLog.Close()
	// Standard log package now routes through slog.
	log.Printf("praetor %s starting", version)

	// Ensure config dir and default config exist.
	cfgFile := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			log.Fatalf("creating config dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(configDir, "scripts"), 0755); err != nil {
			log.Printf("creating scripts dir: %v", err)
		}
		// Write default config.
		defaults := config.Defaults()
		if err := config.Save(defaults, cfgFile); err != nil {
			log.Fatalf("writing default config: %v", err)
		}
		log.Printf("Created default config at %s", cfgFile)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	// Build script directories list, expanding ~ and env vars.
	scriptDirs := make([]string, 0, len(cfg.Scripts))
	for _, dir := range cfg.Scripts {
		scriptDirs = append(scriptDirs, expandPath(dir))
	}
	if len(scriptDirs) == 0 {
		scriptDirs = []string{filepath.Join(configDir, "scripts")}
	}
	creds := &session.KeyringStore{}

	gc, err := client.NewClient(cfg, scriptDirs, dataDir, creds)
	if err != nil {
		log.Fatalf("creating client: %v", err)
	}

	// Session transcript logging.
	logDir := sessionsDir
	if cfg.Logging.Session.Path != "" {
		logDir = cfg.Logging.Session.Path
	}
	sessLog, err := client.NewSessionLogger(cfg.Logging.Session.Enabled, logDir)
	if err != nil {
		log.Printf("session logger: %v", err)
	} else {
		defer sessLog.Close()
	}

	// Determine initial state: if accounts exist, show account selection.
	accounts, err := creds.ListAccounts()
	if err != nil {
		log.Printf("failed to list accounts: %v", err)
		accounts = nil
	}

	gc.Settings.EchoTyped = cfg.UI.EchoTyped
	gc.Settings.EchoScript = cfg.UI.EchoScript

	// Desktop notifications.
	desktopNotify := client.NewDesktopNotifier(cfg.Notifications.Desktop)

	app := ui.NewApp(cfg.UI.SidebarOpen, cfg.UI.DefaultTab, cfg.UI.Scrollback, accounts, cfg.UI.SidebarWidth, cfg.UI.MinimapScale, cfg.UI.MinimapHeight, cfg.UI.QuickCycleModes, cfg.Highlights, *debugFlag, cfg.UI.ColorWords, cfg.UI.CustomTabs, version, cfg.Reconnect.Enabled, cfg.UI.HideIPs, cfg.UI.EchoTyped, cfg.UI.EchoScript, cfg.Logging.Session.Enabled, logDir, scriptDirs, cfg.Commands.HighPriority, cfg.Notifications.Desktop)

	w := wrapper{app: app, gc: gc, cfg: cfg, cfgPath: cfgFile, dataDir: dataDir, configDir: configDir, desktopNotify: desktopNotify}
	p := tea.NewProgram(w, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Bridge game events to the Bubbletea program. We drain all available
	// events from the channel into a single batch so Bubbletea renders once
	// per burst instead of once per line.
	go func() {
		for event := range gc.Events() {
			batch := []types.Event{event}
			// Drain any additional events already queued.
		drain:
			for {
				select {
				case ev, ok := <-gc.Events():
					if !ok {
						break drain
					}
					batch = append(batch, ev)
				default:
					break drain
				}
			}
			// Side effects (logging, notifications) for the whole batch.
			for _, ev := range batch {
				switch e := ev.(type) {
				case types.GameTextEvent:
					if sessLog != nil {
						sessLog.Log(e.Timestamp, e.Text)
					}
					desktopNotify.CheckText(e.Text)
				case types.SKOOTUpdateEvent:
					if e.Health != nil {
						desktopNotify.CheckHealth(*e.Health)
					}
					if e.Fatigue != nil {
						desktopNotify.CheckFatigue(*e.Fatigue)
					}
				case types.ModeChangeEvent:
					desktopNotify.Prune()
				}
			}
			p.Send(ui.EventMsg{Events: batch})
		}
	}()

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
