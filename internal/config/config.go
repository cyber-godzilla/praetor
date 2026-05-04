package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	Reconnect     ReconnectConfig     `yaml:"reconnect"`
	Commands      CommandsConfig      `yaml:"commands"`
	Scripts       []string            `yaml:"scripts"`
	UI            UIConfig            `yaml:"ui"`
	Highlights    []HighlightConfig   `yaml:"highlights"`
	Kudos         KudosConfig         `yaml:"kudos"`
	Ignorelist    Ignorelist          `yaml:"ignorelist"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Logging       LoggingConfig       `yaml:"logging"`
}

type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"`
	LoginURL string `yaml:"login_url"`
}

type ReconnectConfig struct {
	Enabled           bool     `yaml:"enabled"`
	InitialDelay      Duration `yaml:"initial_delay"`
	MaxDelay          Duration `yaml:"max_delay"`
	BackoffMultiplier int      `yaml:"backoff_multiplier"`
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
	Path    string `yaml:"path"` // empty = XDG_DATA_HOME/praetor/sessions/
}

type UIConfig struct {
	// DisplayMode controls how the minimap/compass/vitals are shown:
	//   "sidebar" — vertical strip down the right (default)
	//   "topbar"  — horizontal strip across the top
	//   "off"     — game pane only, sidebar/topbar hidden
	// Migrated from the legacy sidebar_open bool by migrateLegacyDisplay.
	DisplayMode     string            `yaml:"display_mode"`
	DefaultTab      string            `yaml:"default_tab"`
	Scrollback      int               `yaml:"scrollback"`
	SidebarWidth    int               `yaml:"sidebar_width"`
	MinimapScale    float64           `yaml:"minimap_scale"`
	MinimapHeight   int               `yaml:"minimap_height"`
	QuickCycleModes []string          `yaml:"quick_cycle_modes"`
	ColorWords      bool              `yaml:"color_words"`
	EchoTyped       bool              `yaml:"echo_typed_commands"`
	EchoScript      bool              `yaml:"echo_script_commands"`
	HideIPs         bool              `yaml:"hide_ips"`
	CustomTabs      []CustomTabConfig `yaml:"custom_tabs"`
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

func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host:     "game.eternalcitygame.com",
			Port:     8080,
			Protocol: "ws",
			LoginURL: "https://login.eternalcitygame.com/login.php",
		},
		Reconnect: ReconnectConfig{
			Enabled:           true,
			InitialDelay:      Duration{1 * time.Second},
			MaxDelay:          Duration{60 * time.Second},
			BackoffMultiplier: 2,
		},
		Commands: CommandsConfig{
			DefaultDelay: Duration{1000 * time.Millisecond},
			MinInterval:  Duration{500 * time.Millisecond},
			MaxQueueSize: 20,
			HighPriority: []string{},
		},
		Scripts: []string{},
		UI: UIConfig{
			DisplayMode:     "sidebar",
			DefaultTab:      "all",
			Scrollback:      5000,
			SidebarWidth:    40,
			MinimapScale:    0.8,
			MinimapHeight:   12,
			QuickCycleModes: []string{"disable"},
			EchoTyped:       true,
			EchoScript:      true,
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
	}
}

// Save writes the config to the given path.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
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

	// Reconnect
	if c.Reconnect.BackoffMultiplier < 1 {
		c.Reconnect.BackoffMultiplier = 2
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
		c.UI.MinimapScale = 0.8
	}
	if c.UI.MinimapHeight < 4 {
		c.UI.MinimapHeight = 12
	}
	if len(c.UI.QuickCycleModes) == 0 {
		c.UI.QuickCycleModes = []string{"disable"}
	}

	// Highlights
	validStyles := map[string]bool{"red": true, "gold": true, "green": true, "blue": true}
	for i := range c.Highlights {
		if !validStyles[c.Highlights[i].Style] {
			c.Highlights[i].Style = "gold"
		}
	}

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
