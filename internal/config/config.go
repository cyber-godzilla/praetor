package config

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

// saveMu serializes Save so two overlapping writers (GUI setter goroutines,
// which Wails dispatches concurrently) can't interleave their file writes.
var saveMu sync.Mutex

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Commands      CommandsConfig      `yaml:"commands"`
	Credentials   CredentialsConfig   `yaml:"credentials"`
	Scripts       []string            `yaml:"scripts"`
	UI            UIConfig            `yaml:"ui"`
	Highlights    []HighlightConfig   `yaml:"highlights"`
	Kudos         KudosConfig         `yaml:"kudos"`
	Ignorelist    Ignorelist          `yaml:"ignorelist"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Logging       LoggingConfig       `yaml:"logging"`
	Updates       UpdatesConfig       `yaml:"updates"`
}

// UpdatesConfig controls the GUI's startup check against GitHub releases.
type UpdatesConfig struct {
	Check bool `yaml:"check"` // notify when a newer release exists
}

// CredentialsConfig selects the secure account credential backend. Backend is
// deliberately explicit: Praetor never falls back from a failed keyring to a
// plaintext or file-backed store.
type CredentialsConfig struct {
	Backend       string                         `yaml:"backend"`
	EncryptedFile EncryptedFileCredentialsConfig `yaml:"encrypted_file"`
}

// EncryptedFileCredentialsConfig controls the headless-service credential
// store. Path may be empty to use STATE_DIR/credentials/credentials.enc. KeyEnv
// names the environment variable containing a base64-encoded 32-byte key; the
// key itself is never written to config.yaml.
type EncryptedFileCredentialsConfig struct {
	Path   string `yaml:"path"`
	KeyEnv string `yaml:"key_env"`
}

type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"`
	LoginURL string `yaml:"login_url"`
}

type CommandsConfig struct {
	DefaultDelay Duration `yaml:"default_delay"`
	MinInterval  Duration `yaml:"min_interval"`
	MaxQueueSize int      `yaml:"max_queue_size"`
	HighPriority []string `yaml:"high_priority"`
}

type HighlightConfig struct {
	Pattern string `yaml:"pattern"`
	Style   string `yaml:"style"` // red, gold, green, blue
	Active  bool   `yaml:"active"`
}

// KudosConfig stores favorites and queued kudos messages, persisted in
// config.yaml. Favorites is a sorted list of character names. Queue is a
// FIFO of pending kudos messages.
type KudosConfig struct {
	Favorites []string          `yaml:"favorites"`
	Queue     []KudosQueueEntry `yaml:"queue"`
}

// KudosQueueEntry is a single queued kudos message.
type KudosQueueEntry struct {
	Name    string `yaml:"name"`
	Message string `yaml:"message"`
}

// HasFavorite reports whether name (case-insensitive, trim-insensitive)
// is already in the Favorites list.
func (k *KudosConfig) HasFavorite(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return false
	}
	for _, f := range k.Favorites {
		if strings.ToLower(f) == target {
			return true
		}
	}
	return false
}

// AddFavorite inserts name into Favorites if not already present
// (case-insensitive). The list is re-sorted case-insensitively.
// Returns true if the entry was newly added, false if it was a duplicate
// or empty after trimming.
func (k *KudosConfig) AddFavorite(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	if k.HasFavorite(trimmed) {
		return false
	}
	k.Favorites = append(k.Favorites, trimmed)
	sort.SliceStable(k.Favorites, func(i, j int) bool {
		return strings.ToLower(k.Favorites[i]) < strings.ToLower(k.Favorites[j])
	})
	return true
}

// RemoveFavoriteAt removes the favorite at index i. Out-of-range is a no-op.
func (k *KudosConfig) RemoveFavoriteAt(i int) {
	if i < 0 || i >= len(k.Favorites) {
		return
	}
	k.Favorites = append(k.Favorites[:i], k.Favorites[i+1:]...)
}

// AddQueueEntry appends a new queue entry with name and message both trimmed.
// Empty (post-trim) name or message is a no-op.
func (k *KudosConfig) AddQueueEntry(name, message string) {
	n := strings.TrimSpace(name)
	m := strings.TrimSpace(message)
	if n == "" || m == "" {
		return
	}
	k.Queue = append(k.Queue, KudosQueueEntry{Name: n, Message: m})
}

// RemoveQueueAt removes the queue entry at index i. Out-of-range is a no-op.
func (k *KudosConfig) RemoveQueueAt(i int) {
	if i < 0 || i >= len(k.Queue) {
		return
	}
	k.Queue = append(k.Queue[:i], k.Queue[i+1:]...)
}

type Ignorelist struct {
	OOC   []string `yaml:"ooc"`
	Think []string `yaml:"think"`
}

type NotificationsConfig struct {
	Desktop DesktopNotificationsConfig `yaml:"desktop"`
}

type DesktopNotificationsConfig struct {
	HealthBelow  ThresholdConfig       `yaml:"health_below"`
	FatigueBelow ThresholdConfig       `yaml:"fatigue_below"`
	Patterns     []NotifyPatternConfig `yaml:"patterns"`
}

type ThresholdConfig struct {
	Enabled   bool `yaml:"enabled"`
	Threshold int  `yaml:"threshold"`
}

type NotifyPatternConfig struct {
	Pattern string `yaml:"pattern"`
	Title   string `yaml:"title"`   // custom notification title
	Message string `yaml:"message"` // custom notification message
	Enabled bool   `yaml:"enabled"`
}

type LoggingConfig struct {
	App     AppLoggingConfig     `yaml:"app"`
	Session SessionLoggingConfig `yaml:"session"`
}

type AppLoggingConfig struct {
	Level     string `yaml:"level"` // debug, info, warn, error
	MaxSizeMB int    `yaml:"max_size_mb"`
}

type SessionLoggingConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"` // empty = XDG_CONFIG_HOME/praetor/logs/
}

type UIConfig struct {
	// DisplayMode controls how the minimap/compass/vitals are shown:
	//   "sidebar" — vertical strip down the right (default)
	//   "topbar"  — horizontal strip across the top
	//   "off"     — game pane only, sidebar/topbar hidden
	// Migrated from the legacy sidebar_open bool by migrateLegacyDisplay.
	DisplayMode     string   `yaml:"display_mode"`
	DefaultTab      string   `yaml:"default_tab"`
	Scrollback      int      `yaml:"scrollback"`
	SidebarWidth    int      `yaml:"sidebar_width"`
	MinimapScale    float64  `yaml:"minimap_scale"`
	MinimapHeight   int      `yaml:"minimap_height"`
	CompassScale    float64  `yaml:"compass_scale"`
	OutputFontSize  int      `yaml:"output_font_size"`
	CRTScanlines    bool     `yaml:"crt_scanlines"`
	CRTRoll         bool     `yaml:"crt_roll"`
	CRTBloom        bool     `yaml:"crt_bloom"`
	QuickCycleModes []string `yaml:"quick_cycle_modes"`
	ColorWords      bool     `yaml:"color_words"`
	EchoTyped       bool     `yaml:"echo_typed_commands"`
	EchoScript      bool     `yaml:"echo_script_commands"`
	HideIPs         bool     `yaml:"hide_ips"`
	// InputSpellcheck enables the webview's native spellchecker on the GUI
	// command input (red squiggles under misspelled words while composing says
	// and emotes). Engine support varies by platform webview.
	InputSpellcheck bool `yaml:"input_spellcheck"`
	// Mobile web presentation settings are persisted with the shared UI config,
	// but the native Wails and TUI shells deliberately ignore them.
	MobileOutputFontSize        int  `yaml:"mobile_output_font_size"`
	MobileShowToolbar           bool `yaml:"mobile_show_toolbar"`
	MobileShowTabBar            bool `yaml:"mobile_show_tab_bar"`
	MobileHideNavigationOnInput bool `yaml:"mobile_hide_navigation_on_input"`
	MobileLowercaseFirstLetter  bool `yaml:"mobile_lowercase_first_letter"`
	// NumpadNavigation controls the GUI numpad-walking behavior:
	//   "numlock" — move when NumLock is off; type digits when on (default)
	//   "always"  — numpad always sends movement (needed on macOS, which has
	//               no NumLock; the numpad can no longer type digits)
	//   "off"     — numpad navigation disabled
	NumpadNavigation string            `yaml:"numpad_navigation"`
	CustomTabs       []CustomTabConfig `yaml:"custom_tabs"`
	ActionSets       []ActionSet       `yaml:"action_sets"`
}

type CustomTabConfig struct {
	Name         string          `yaml:"name"`
	Visible      bool            `yaml:"visible"`
	EchoCommands bool            `yaml:"echo_commands"` // only meaningful when tab is exclude-only
	Rules        []TabRuleConfig `yaml:"rules"`
}

type TabRuleConfig struct {
	Pattern string `yaml:"pattern"` // supports * and ? wildcards
	Include bool   `yaml:"include"` // true = "does match", false = "does not match"
	Active  bool   `yaml:"active"`
}

// ActionSet is a named group of quick-action buttons shown in the sidebar
// Actions tab. Users manage sets/buttons via the Action Sets editor.
type ActionSet struct {
	Name    string         `yaml:"name"`
	Buttons []ActionButton `yaml:"buttons"`
}

// ActionButton is a single quick-action: a label and the game command it sends.
type ActionButton struct {
	Label   string `yaml:"label"`
	Command string `yaml:"command"`
}

func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "game.eternalcitygame.com",
			Port:     8080,
			Protocol: "ws",
			LoginURL: "https://login.eternalcitygame.com/login.php",
		},
		Commands: CommandsConfig{
			DefaultDelay: Duration{1000 * time.Millisecond},
			MinInterval:  Duration{500 * time.Millisecond},
			MaxQueueSize: 20,
			HighPriority: []string{},
		},
		Credentials: CredentialsConfig{
			Backend: "keyring",
			EncryptedFile: EncryptedFileCredentialsConfig{
				Path:   "",
				KeyEnv: "PRAETOR_CREDENTIALS_KEY",
			},
		},
		Scripts: []string{},
		UI: UIConfig{
			DisplayMode:          "sidebar",
			DefaultTab:           "all",
			Scrollback:           5000,
			SidebarWidth:         40,
			MinimapScale:         1.0,
			MinimapHeight:        12,
			CompassScale:         1.0,
			OutputFontSize:       14,
			CRTScanlines:         true,
			CRTRoll:              true,
			CRTBloom:             true,
			QuickCycleModes:      []string{"disable"},
			EchoTyped:            true,
			EchoScript:           true,
			InputSpellcheck:      true,
			MobileOutputFontSize: 14,
			MobileShowToolbar:    true,
			MobileShowTabBar:     true,
			NumpadNavigation:     "numlock",
		},
		Highlights: []HighlightConfig{},
		Kudos: KudosConfig{
			Favorites: []string{},
			Queue:     []KudosQueueEntry{},
		},
		Notifications: NotificationsConfig{
			Desktop: DesktopNotificationsConfig{
				HealthBelow: ThresholdConfig{
					Enabled:   true,
					Threshold: 25,
				},
				FatigueBelow: ThresholdConfig{
					Enabled:   false,
					Threshold: 10,
				},
				Patterns: []NotifyPatternConfig{},
			},
		},
		Logging: LoggingConfig{
			App: AppLoggingConfig{
				Level:     "info",
				MaxSizeMB: 5,
			},
			Session: SessionLoggingConfig{
				Enabled: true,
				Path:    "",
			},
		},
		Updates: UpdatesConfig{
			Check: true,
		},
	}
}

// Save writes the config to the given path atomically: it marshals, writes a
// temp file in the same directory, fsyncs it, then renames it over the target.
// A crash or power loss mid-write therefore leaves either the old complete file
// or the new one — never a truncated config.yaml that Load hard-fails on.
// Concurrent Saves are serialized. Multi-instance play is last-writer-wins, but
// the file is never torn.
func Save(cfg *Config, path string) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Preserve the existing file's permissions; default to 0644 for a new file.
	perm := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}

	return atomicfile.Write(path, data, perm)
}

func Load(path string) (*Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	migrateLegacyEcho(cfg, data)
	migrateLegacyDisplay(cfg, data)
	migrateMobileOutputFontSize(cfg, data)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// migrateMobileOutputFontSize preserves the pre-split behavior for existing
// configurations. Before this field existed, mobile and desktop web output
// both used output_font_size; copy that value only when the mobile key is
// absent. Once saved, the two values remain independent.
func migrateMobileOutputFontSize(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	if _, exists := ui["mobile_output_font_size"]; !exists {
		// Match the validation that output_font_size received before the split,
		// including its fallback for legacy values below the desktop minimum.
		size := cfg.UI.OutputFontSize
		if size < 8 {
			size = 14
		} else if size > 40 {
			size = 40
		}
		cfg.UI.MobileOutputFontSize = size
	}
}

// migrateLegacyEcho copies the deprecated ui.echo_commands value into the
// split EchoTyped/EchoScript fields when the new keys are absent.
func migrateLegacyEcho(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	legacy, hasLegacy := ui["echo_commands"]
	if !hasLegacy {
		return
	}
	b, ok := legacy.(bool)
	if !ok {
		return
	}
	if _, hasTyped := ui["echo_typed_commands"]; !hasTyped {
		cfg.UI.EchoTyped = b
	}
	if _, hasScript := ui["echo_script_commands"]; !hasScript {
		cfg.UI.EchoScript = b
	}
}

// migrateLegacyDisplay translates the deprecated ui.sidebar_open bool
// into the new ui.display_mode string when the new key is absent.
// sidebar_open=true  → display_mode="sidebar"
// sidebar_open=false → display_mode="off"
// Validate normalizes the result; unknown values fall back to "sidebar".
func migrateLegacyDisplay(cfg *Config, data []byte) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	ui, ok := raw["ui"].(map[string]interface{})
	if !ok {
		return
	}
	if _, hasNew := ui["display_mode"]; hasNew {
		return
	}
	legacy, hasLegacy := ui["sidebar_open"]
	if !hasLegacy {
		return
	}
	b, ok := legacy.(bool)
	if !ok {
		return
	}
	if b {
		cfg.UI.DisplayMode = "sidebar"
	} else {
		cfg.UI.DisplayMode = "off"
	}
}

// Validate checks the config for invalid or out-of-range values,
// clamping to sensible defaults where possible and returning an
// error only for truly broken configuration.
// TransportWarnings returns non-fatal advisories about cleartext transport: an
// http:// login URL POSTs the raw password unencrypted, and the ws:// protocol
// sends the credential-equivalent cookies, the MD5 handshake, and all game
// traffic in the clear. These are warnings, not validation errors — the shipped
// default is ws:// because the game server may not offer TLS, so making it an
// error would brick the default config. The shells log these at startup.
func (c *Config) TransportWarnings() []string {
	var warnings []string
	if strings.HasPrefix(strings.ToLower(c.Server.LoginURL), "http://") {
		warnings = append(warnings, fmt.Sprintf(
			"server.login_url uses cleartext http:// (%s) — your password is sent unencrypted; use https:// if the server supports it",
			c.Server.LoginURL))
	}
	if c.Server.Protocol == "ws" {
		warnings = append(warnings,
			"server.protocol is ws:// (cleartext) — session cookies and game traffic are unencrypted; use wss:// if the server supports it")
	}
	return warnings
}

func (c *Config) Validate() error {
	// Server
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	if c.Server.Protocol != "ws" && c.Server.Protocol != "wss" {
		return fmt.Errorf("server.protocol must be 'ws' or 'wss', got %q", c.Server.Protocol)
	}
	if c.Server.LoginURL == "" {
		return fmt.Errorf("server.login_url is required")
	}

	// Commands
	if c.Commands.DefaultDelay.Duration < 100*time.Millisecond {
		c.Commands.DefaultDelay = Duration{900 * time.Millisecond}
	}
	if c.Commands.MinInterval.Duration < 50*time.Millisecond {
		c.Commands.MinInterval = Duration{400 * time.Millisecond}
	}
	if c.Commands.MaxQueueSize < 1 {
		c.Commands.MaxQueueSize = 20
	}

	// Credentials. Backends are never inferred from runtime availability: a
	// missing keyring remains a visible keyring error, and an encrypted file
	// requires its own independently supplied key.
	switch c.Credentials.Backend {
	case "keyring", "encrypted_file", "disabled":
	default:
		return fmt.Errorf("credentials.backend must be 'keyring', 'encrypted_file', or 'disabled', got %q", c.Credentials.Backend)
	}
	if c.Credentials.EncryptedFile.KeyEnv == "" {
		c.Credentials.EncryptedFile.KeyEnv = "PRAETOR_CREDENTIALS_KEY"
	}
	for i, r := range c.Credentials.EncryptedFile.KeyEnv {
		if (i == 0 && r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')) ||
			(i > 0 && r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9')) {
			return fmt.Errorf("credentials.encrypted_file.key_env is not a valid environment variable name")
		}
	}

	// UI
	validDisplayModes := map[string]bool{"sidebar": true, "topbar": true, "off": true}
	if !validDisplayModes[c.UI.DisplayMode] {
		c.UI.DisplayMode = "sidebar"
	}
	validTabs := map[string]bool{"all": true, "general": true, "combat": true, "social": true, "metrics": true}
	if !validTabs[c.UI.DefaultTab] {
		c.UI.DefaultTab = "all"
	}
	if c.UI.Scrollback < 100 {
		c.UI.Scrollback = 5000
	}
	if c.UI.SidebarWidth < 20 {
		c.UI.SidebarWidth = 40
	}
	if c.UI.MinimapScale <= 0 {
		c.UI.MinimapScale = 1.0
	}
	if c.UI.MinimapHeight < 4 {
		c.UI.MinimapHeight = 12
	}
	if c.UI.CompassScale <= 0 {
		c.UI.CompassScale = 1.0
	}
	if c.UI.OutputFontSize < 8 {
		c.UI.OutputFontSize = 14
	} else if c.UI.OutputFontSize > 40 {
		c.UI.OutputFontSize = 40
	}
	if c.UI.MobileOutputFontSize < 6 {
		c.UI.MobileOutputFontSize = 6
	} else if c.UI.MobileOutputFontSize > 40 {
		c.UI.MobileOutputFontSize = 40
	}
	if len(c.UI.QuickCycleModes) == 0 {
		c.UI.QuickCycleModes = []string{"disable"}
	}
	// Numpad navigation mode: fall back to "numlock" for empty (pre-existing
	// configs) or unrecognized values.
	switch c.UI.NumpadNavigation {
	case "numlock", "always", "off":
	default:
		c.UI.NumpadNavigation = "numlock"
	}

	// Highlights
	validStyles := map[string]bool{"red": true, "gold": true, "green": true, "blue": true}
	kept := c.Highlights[:0]
	for i := range c.Highlights {
		// Drop empty/whitespace-only patterns: a case-insensitive empty-substring
		// match highlights every line. Reachable only by hand-editing config.yaml.
		if strings.TrimSpace(c.Highlights[i].Pattern) == "" {
			log.Printf("[CONFIG] dropping highlight with empty pattern")
			continue
		}
		if !validStyles[c.Highlights[i].Style] {
			c.Highlights[i].Style = "gold"
		}
		kept = append(kept, c.Highlights[i])
	}
	c.Highlights = kept

	// Notifications
	if c.Notifications.Desktop.HealthBelow.Threshold < 0 || c.Notifications.Desktop.HealthBelow.Threshold > 100 {
		c.Notifications.Desktop.HealthBelow.Threshold = 25
	}
	if c.Notifications.Desktop.FatigueBelow.Threshold < 0 || c.Notifications.Desktop.FatigueBelow.Threshold > 100 {
		c.Notifications.Desktop.FatigueBelow.Threshold = 10
	}

	// Logging
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.App.Level] {
		c.Logging.App.Level = "info"
	}
	if c.Logging.App.MaxSizeMB < 1 {
		c.Logging.App.MaxSizeMB = 5
	}

	return nil
}
