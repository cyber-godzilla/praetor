package gui

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyber-godzilla/praetor/internal/client"
	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/logging"
	"github.com/cyber-godzilla/praetor/internal/notes"
	"github.com/cyber-godzilla/praetor/internal/session"
)

// Deps bundles everything the GUI needs, constructed once at startup. It
// mirrors the wiring in cmd/praetor/main.go so both frontends share identical
// setup (XDG dirs, config, logging, client, notifier).
type Deps struct {
	Client        *client.Client
	Config        *config.Config
	ConfigPath    string
	ConfigDir     string
	DataDir       string
	StateDir      string
	SessionsDir   string
	Creds         session.CredentialStore
	SessionLog    *client.SessionLogger
	DesktopNotify *client.DesktopNotifier
	Clipboard     Clipboard
	Dialogs       Dialogs
	ScriptDirs    []string
	Notes         *notes.Store
	Version       string
	Debug         bool

	appLog *logging.Logger
}

// Close releases resources held by Deps (engine, log files, session log).
func (d *Deps) Close() {
	// Close the engine first: it flushes persistent state synchronously, so the
	// last few seconds of state (within the 5s debounce window) survive a quit.
	if d.Client != nil && d.Client.Engine != nil {
		d.Client.Engine.Close()
	}
	if d.SessionLog != nil {
		d.SessionLog.Close()
	}
	if d.appLog != nil {
		d.appLog.Close()
	}
}

// Bootstrap performs the same startup sequence as the TUI: resolve XDG dirs,
// ensure config exists, load it, set up logging, and wire the client,
// credential store, session logger, and desktop notifier.
func Bootstrap(version string, debug bool) (*Deps, error) {
	configDir := appDir("PRAETOR_CONFIG_DIR", "XDG_CONFIG_HOME", ".config", "praetor")
	dataDir := appDir("PRAETOR_DATA_DIR", "XDG_DATA_HOME", ".local/share", "praetor")
	stateDir := appDir("PRAETOR_STATE_DIR", "XDG_STATE_HOME", ".local/state", "praetor")
	sessionsDir := filepath.Join(configDir, "logs")

	logLevel := "info"
	if debug {
		logLevel = "debug"
	}
	appLog, err := logging.New(stateDir, "tec.log", logLevel, 5)
	if err != nil {
		return nil, err
	}

	cfgFile := filepath.Join(configDir, "config.yaml")
	if _, statErr := os.Stat(cfgFile); os.IsNotExist(statErr) {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Join(configDir, "scripts"), 0o755); err != nil {
			return nil, err
		}
		if err := config.Save(config.Defaults(), cfgFile); err != nil {
			return nil, err
		}
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return nil, err
	}
	for _, w := range cfg.TransportWarnings() {
		log.Printf("[CONFIG] %s", w)
	}

	scriptDirs := make([]string, 0, len(cfg.Scripts))
	for _, dir := range cfg.Scripts {
		scriptDirs = append(scriptDirs, expandPath(dir))
	}
	if len(scriptDirs) == 0 {
		scriptDirs = []string{filepath.Join(configDir, "scripts")}
	}

	credentialPath := cfg.Credentials.EncryptedFile.Path
	if credentialPath != "" {
		credentialPath = expandPath(credentialPath)
	}
	creds, err := session.NewCredentialStore(session.CredentialStoreOptions{
		Backend:  cfg.Credentials.Backend,
		StateDir: stateDir,
		FilePath: credentialPath,
		KeyEnv:   cfg.Credentials.EncryptedFile.KeyEnv,
	})
	if err != nil {
		appLog.Close()
		return nil, err
	}
	gc, err := client.NewClient(cfg, scriptDirs, dataDir, creds)
	if err != nil {
		return nil, err
	}
	gc.SetIgnoreOOC(cfg.Ignorelist.OOC)
	gc.SetIgnoreThink(cfg.Ignorelist.Think)
	gc.Settings.EchoTyped = cfg.UI.EchoTyped
	gc.Settings.EchoScript = cfg.UI.EchoScript

	logDir := sessionsDir
	if cfg.Logging.Session.Path != "" {
		logDir = expandPath(cfg.Logging.Session.Path)
	}
	sessLog, err := client.NewSessionLogger(cfg.Logging.Session.Enabled, logDir)
	if err != nil {
		// Non-fatal: retain a disabled logger so a corrected path can be
		// applied live from any shell without restarting the process.
		sessLog, _ = client.NewSessionLogger(false, logDir)
	}
	desktopNotify := client.NewDesktopNotifier(cfg.Notifications.Desktop)
	gc.Engine.SetNotificationSink(desktopNotify.Notify)

	notesStore := notes.New(filepath.Join(configDir, "notes"))

	return &Deps{
		Client:        gc,
		Config:        cfg,
		ConfigPath:    cfgFile,
		ConfigDir:     configDir,
		DataDir:       dataDir,
		StateDir:      stateDir,
		SessionsDir:   sessionsDir,
		Creds:         creds,
		SessionLog:    sessLog,
		DesktopNotify: desktopNotify,
		ScriptDirs:    scriptDirs,
		Notes:         notesStore,
		Version:       version,
		Debug:         debug,
		appLog:        appLog,
	}, nil
}

// appDir permits a service deployment to point Praetor at an exact application
// directory. XDG variables name parent directories, so relying on them alone
// always appends /praetor and cannot represent an existing direct layout such
// as /srv/praetor/config. Desktop installs retain the normal XDG behavior when
// the explicit override is absent.
func appDir(overrideEnv, xdgEnv, defaultSuffix, appName string) string {
	if dir := os.Getenv(overrideEnv); dir != "" {
		return filepath.Clean(dir)
	}
	return xdgPath(xdgEnv, defaultSuffix, appName)
}

// xdgPath returns the XDG directory for the given env var, falling back to
// $HOME/<defaultSuffix>, with the app subdirectory appended.
func xdgPath(envVar, defaultSuffix, appName string) string {
	dir := os.Getenv(envVar)
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, defaultSuffix)
	}
	return filepath.Join(dir, appName)
}

// expandPath expands ~ and $ENV references in a path.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	return os.ExpandEnv(path)
}
