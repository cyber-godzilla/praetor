package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestDefaults_AppLoggingIsInfoAndRetentionOff(t *testing.T) {
	cfg := Defaults()
	if cfg.Logging.App.Level != "info" {
		t.Errorf("Level = %q, want info", cfg.Logging.App.Level)
	}
	if cfg.Logging.App.Retain {
		t.Error("Retain = true, want false")
	}
}

func TestValidate_DropsEmptyHighlightPattern(t *testing.T) {
	c := Defaults()
	c.Highlights = []HighlightConfig{
		{Pattern: "gold", Style: "gold", Active: true},
		{Pattern: "", Style: "red", Active: true},
		{Pattern: "   ", Style: "blue", Active: true},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(c.Highlights) != 1 || c.Highlights[0].Pattern != "gold" {
		t.Fatalf("empty-pattern highlights not dropped: %+v", c.Highlights)
	}
}

func TestConfig_TransportWarnings(t *testing.T) {
	// Cleartext on both axes → two warnings.
	c := Defaults()
	c.Server.LoginURL = "http://login.example.com/login.php"
	c.Server.Protocol = "ws"
	if w := c.TransportWarnings(); len(w) != 2 {
		t.Fatalf("http+ws warnings = %d (%v), want 2", len(w), w)
	}

	// Fully secure → no warnings.
	c.Server.LoginURL = "https://login.example.com/login.php"
	c.Server.Protocol = "wss"
	if w := c.TransportWarnings(); len(w) != 0 {
		t.Fatalf("https+wss warnings = %v, want none", w)
	}

	// The shipped default (https login, ws protocol) warns only about ws.
	c.Server.Protocol = "ws"
	if w := c.TransportWarnings(); len(w) != 1 {
		t.Fatalf("https+ws warnings = %v, want 1 (protocol only)", w)
	}
}

func TestSave_IsAtomic_NoLeftoverTempAndPreservesPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Defaults()
	if err := Save(cfg, path); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	// Tighten perms, then save again — perms must be preserved (not reset to 0644).
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := Save(cfg, path); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("perms after Save = %o, want 600 (perms not preserved)", perm)
	}

	// No temp file may be left behind in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "config.yaml" {
			t.Errorf("unexpected leftover file after Save: %q", e.Name())
		}
	}
}

func TestSave_FailedWriteLeavesOriginalIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Defaults()
	cfg.UI.SidebarWidth = 42
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Saving to a path whose directory does not exist must fail without a
	// truncate-in-place, leaving the existing file (a different path) untouched.
	bad := filepath.Join(dir, "nonexistent-subdir", "config.yaml")
	if err := Save(cfg, bad); err == nil {
		t.Fatal("Save to a missing directory unexpectedly succeeded")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != string(original) {
		t.Error("original config was modified by a failed Save to another path")
	}
}

func TestSave_ConcurrentSavesDoNotRace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Defaults()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Save(cfg, path)
		}()
	}
	wg.Wait()

	// The file must be complete and loadable after concurrent saves.
	if _, err := Load(path); err != nil {
		t.Fatalf("config unreadable after concurrent saves: %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: test.example.com
  port: 9090
  protocol: wss
reconnect:
  enabled: true
  initial_delay: 2s
  max_delay: 30s
  backoff_multiplier: 3
commands:
  default_delay: 800ms
  min_interval: 300ms
  max_queue_size: 15
  high_priority:
    - stand
    - app1
ui:
  sidebar_open: false
  default_tab: combat
  scrollback: 3000
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Host != "test.example.com" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "test.example.com")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Commands.DefaultDelay.String() != "800ms" {
		t.Errorf("Commands.DefaultDelay = %v, want 800ms", cfg.Commands.DefaultDelay)
	}
	if cfg.UI.DisplayMode != "off" {
		t.Errorf("UI.DisplayMode = %q, want %q (migrated from sidebar_open: false)", cfg.UI.DisplayMode, "off")
	}
	if len(cfg.Commands.HighPriority) != 2 {
		t.Errorf("HighPriority len = %d, want 2", len(cfg.Commands.HighPriority))
	}
}

func TestLoadConfigDisplayMode(t *testing.T) {
	cases := []struct {
		name string
		ui   string
		want string
	}{
		{"explicit topbar", "  display_mode: topbar\n", "topbar"},
		{"explicit sidebar", "  display_mode: sidebar\n", "sidebar"},
		{"explicit off", "  display_mode: off\n", "off"},
		{"legacy sidebar_open true", "  sidebar_open: true\n", "sidebar"},
		{"legacy sidebar_open false", "  sidebar_open: false\n", "off"},
		{"unknown value falls back to sidebar", "  display_mode: bogus\n", "sidebar"},
	}
	header := `server:
  host: game.eternalcitygame.com
  port: 8080
  protocol: ws
  login_url: https://login.eternalcitygame.com/login.php
ui:
`
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(header+tc.ui), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.UI.DisplayMode != tc.want {
				t.Errorf("DisplayMode = %q, want %q", cfg.UI.DisplayMode, tc.want)
			}
		})
	}
}

func TestLoadConfigMinimapFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
ui:
  sidebar_width: 40
  minimap_scale: 0.8
  minimap_height: 12
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.UI.SidebarWidth != 40 {
		t.Errorf("UI.SidebarWidth = %d, want 40", cfg.UI.SidebarWidth)
	}
	if cfg.UI.MinimapScale != 0.8 {
		t.Errorf("UI.MinimapScale = %f, want 0.8", cfg.UI.MinimapScale)
	}
	if cfg.UI.MinimapHeight != 12 {
		t.Errorf("UI.MinimapHeight = %d, want 12", cfg.UI.MinimapHeight)
	}
}

func TestLoadConfigMobileWebSettings(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(`
ui:
  output_font_size: 18
  mobile_output_font_size: 6
  mobile_show_toolbar: false
  mobile_show_tab_bar: false
  mobile_hide_navigation_on_input: true
  mobile_lowercase_first_letter: true
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UI.MobileShowToolbar {
		t.Error("MobileShowToolbar = true, want false")
	}
	if cfg.UI.MobileShowTabBar {
		t.Error("MobileShowTabBar = true, want false")
	}
	if cfg.UI.OutputFontSize != 18 || cfg.UI.MobileOutputFontSize != 6 {
		t.Errorf("font sizes = desktop %d, mobile %d; want 18 and 6", cfg.UI.OutputFontSize, cfg.UI.MobileOutputFontSize)
	}
	if !cfg.UI.MobileHideNavigationOnInput {
		t.Error("MobileHideNavigationOnInput = false, want true")
	}
	if !cfg.UI.MobileLowercaseFirstLetter {
		t.Error("MobileLowercaseFirstLetter = false, want true")
	}
}

func TestLoadConfigMobileWebDefaultsForExistingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("ui:\n  scrollback: 5000\n  output_font_size: 11\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.UI.MobileShowToolbar {
		t.Error("missing mobile_show_toolbar should preserve the existing visible toolbar")
	}
	if !cfg.UI.MobileShowTabBar {
		t.Error("missing mobile_show_tab_bar should preserve the existing visible tab selector")
	}
	if cfg.UI.MobileHideNavigationOnInput {
		t.Error("missing mobile_hide_navigation_on_input should preserve visible navigation")
	}
	if cfg.UI.MobileLowercaseFirstLetter {
		t.Error("missing mobile_lowercase_first_letter should not rewrite command input")
	}
	if cfg.UI.MobileOutputFontSize != 11 {
		t.Errorf("missing mobile_output_font_size = %d, want existing output_font_size 11", cfg.UI.MobileOutputFontSize)
	}
}

func TestLoadConfigMobileFontMigrationUsesValidatedDesktopValue(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("ui:\n  output_font_size: 6\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.UI.OutputFontSize != 14 || cfg.UI.MobileOutputFontSize != 14 {
		t.Errorf("migrated sizes = desktop %d, mobile %d; want validated legacy value 14", cfg.UI.OutputFontSize, cfg.UI.MobileOutputFontSize)
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	cfg := Defaults()
	cfg.Server.Host = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty server.host")
	}

	cfg = Defaults()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port 0")
	}

	cfg = Defaults()
	cfg.Server.Protocol = "http"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid protocol")
	}

	cfg = Defaults()
	cfg.Server.LoginURL = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty login_url")
	}
}

func TestCredentialBackendDefaultsAndValidation(t *testing.T) {
	cfg := Defaults()
	if cfg.Credentials.Backend != "keyring" || cfg.Credentials.EncryptedFile.KeyEnv != "PRAETOR_CREDENTIALS_KEY" {
		t.Fatalf("credential defaults = %+v", cfg.Credentials)
	}
	for _, backend := range []string{"keyring", "encrypted_file", "disabled"} {
		cfg := Defaults()
		cfg.Credentials.Backend = backend
		if err := cfg.Validate(); err != nil {
			t.Errorf("backend %q rejected: %v", backend, err)
		}
	}
	cfg = Defaults()
	cfg.Credentials.Backend = "automatic"
	if err := cfg.Validate(); err == nil {
		t.Fatal("implicit/fallback credential backend was accepted")
	}
	cfg = Defaults()
	cfg.Credentials.EncryptedFile.KeyEnv = "BAD-NAME"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid credential key environment name was accepted")
	}
}

func TestExistingConfigDefaultsToKeyringWithoutWritingASecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: game.example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Credentials.Backend != "keyring" {
		t.Fatalf("backend = %q, want keyring", cfg.Credentials.Backend)
	}
	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "credentials:") || !strings.Contains(text, "backend: keyring") || strings.Contains(text, "PRAETOR_CREDENTIALS_KEY=") {
		t.Fatalf("saved credential configuration is wrong:\n%s", text)
	}
}

func TestValidate_ClampsValues(t *testing.T) {
	cfg := Defaults()
	cfg.UI.Scrollback = 10
	cfg.UI.SidebarWidth = 5
	cfg.UI.MinimapScale = -1
	cfg.UI.MinimapHeight = 1
	cfg.UI.DefaultTab = "invalid"
	cfg.UI.QuickCycleModes = nil
	cfg.Commands.MaxQueueSize = 0
	cfg.Logging.App.Level = "bogus"
	cfg.Logging.App.MaxSizeMB = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if cfg.UI.Scrollback != 5000 {
		t.Errorf("Scrollback = %d, want 5000", cfg.UI.Scrollback)
	}
	if cfg.UI.SidebarWidth != 40 {
		t.Errorf("SidebarWidth = %d, want 40", cfg.UI.SidebarWidth)
	}
	if cfg.UI.MinimapScale != 1.0 {
		t.Errorf("MinimapScale = %f, want 1.0", cfg.UI.MinimapScale)
	}
	if cfg.UI.MinimapHeight != 12 {
		t.Errorf("MinimapHeight = %d, want 12", cfg.UI.MinimapHeight)
	}
	if cfg.UI.DefaultTab != "all" {
		t.Errorf("DefaultTab = %q, want 'all'", cfg.UI.DefaultTab)
	}
	if len(cfg.UI.QuickCycleModes) != 1 {
		t.Errorf("QuickCycleModes len = %d, want 1", len(cfg.UI.QuickCycleModes))
	}
	if cfg.Commands.MaxQueueSize != 20 {
		t.Errorf("MaxQueueSize = %d, want 20", cfg.Commands.MaxQueueSize)
	}
	if cfg.Logging.App.Level != "info" {
		t.Errorf("Level = %q, want 'info'", cfg.Logging.App.Level)
	}
	if cfg.Logging.App.MaxSizeMB != 5 {
		t.Errorf("MaxSizeMB = %d, want 5", cfg.Logging.App.MaxSizeMB)
	}
	if cfg.Logging.App.Retain {
		t.Error("Retain = true, want false")
	}
}

func TestValidateMobileOutputFontSizeRange(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{name: "below minimum", value: 5, want: 6},
		{name: "minimum", value: 6, want: 6},
		{name: "maximum", value: 40, want: 40},
		{name: "above maximum", value: 41, want: 40},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.UI.MobileOutputFontSize = test.value
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			if cfg.UI.MobileOutputFontSize != test.want {
				t.Errorf("MobileOutputFontSize = %d, want %d", cfg.UI.MobileOutputFontSize, test.want)
			}
		})
	}
}

func TestValidate_HighlightStyle(t *testing.T) {
	cfg := Defaults()
	cfg.Highlights = []HighlightConfig{
		{Pattern: "test", Style: "invalid", Active: true},
		{Pattern: "ok", Style: "red", Active: true},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if cfg.Highlights[0].Style != "gold" {
		t.Errorf("invalid style should be corrected to 'gold', got %q", cfg.Highlights[0].Style)
	}
	if cfg.Highlights[1].Style != "red" {
		t.Errorf("valid style should be unchanged, got %q", cfg.Highlights[1].Style)
	}
}

func TestValidate_NotificationThresholds(t *testing.T) {
	cfg := Defaults()
	cfg.Notifications.Desktop.HealthBelow.Threshold = 150
	cfg.Notifications.Desktop.FatigueBelow.Threshold = -5

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if cfg.Notifications.Desktop.HealthBelow.Threshold != 25 {
		t.Errorf("HealthBelow threshold = %d, want 25", cfg.Notifications.Desktop.HealthBelow.Threshold)
	}
	if cfg.Notifications.Desktop.FatigueBelow.Threshold != 10 {
		t.Errorf("FatigueBelow threshold = %d, want 10", cfg.Notifications.Desktop.FatigueBelow.Threshold)
	}
}

func TestConfigDefaultsMinimapFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: game.example.com
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.UI.SidebarWidth != 40 {
		t.Errorf("default SidebarWidth = %d, want 40", cfg.UI.SidebarWidth)
	}
	if cfg.UI.MinimapScale != 1.0 {
		t.Errorf("default MinimapScale = %f, want 1.0", cfg.UI.MinimapScale)
	}
	if cfg.UI.MinimapHeight != 12 {
		t.Errorf("default MinimapHeight = %d, want 12", cfg.UI.MinimapHeight)
	}
}

func TestLoadConfig_LegacyEchoCommandsMigratesToBoth(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: game.example.com
ui:
  echo_commands: false
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.UI.EchoTyped {
		t.Error("legacy echo_commands: false should set EchoTyped to false")
	}
	if cfg.UI.EchoScript {
		t.Error("legacy echo_commands: false should set EchoScript to false")
	}
}

func TestLoadConfig_NewEchoKeysHonored(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: game.example.com
ui:
  echo_typed_commands: true
  echo_script_commands: false
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !cfg.UI.EchoTyped {
		t.Error("explicit echo_typed_commands: true not honored")
	}
	if cfg.UI.EchoScript {
		t.Error("explicit echo_script_commands: false not honored")
	}
}

func TestSaveConfig_OmitsLegacyEchoKey(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := Defaults()
	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "echo_typed_commands:") {
		t.Error("saved config missing echo_typed_commands key")
	}
	if !contains(s, "echo_script_commands:") {
		t.Error("saved config missing echo_script_commands key")
	}
	// Legacy key must not be present under ui: section.
	if containsLegacyUIEcho(s) {
		t.Error("saved config should not emit legacy echo_commands under ui:")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// containsLegacyUIEcho returns true if a top-level ui.echo_commands key is
// present (distinct from custom_tabs[].echo_commands).
func containsLegacyUIEcho(s string) bool {
	// Look for "\n  echo_commands:" — 2-space indent = direct child of ui:.
	// Under custom_tabs the indent is 4+ spaces.
	return contains(s, "\n  echo_commands:")
}

func TestIgnorelist_RoundTrip(t *testing.T) {
	cfg := Defaults()
	cfg.Ignorelist.OOC = []string{"xXSephirothXx", "dArKwInG666"}
	cfg.Ignorelist.Think = []string{"Travis", "Andrea"}

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(loaded.Ignorelist.OOC, cfg.Ignorelist.OOC) {
		t.Errorf("OOC mismatch: got %v, want %v", loaded.Ignorelist.OOC, cfg.Ignorelist.OOC)
	}
	if !reflect.DeepEqual(loaded.Ignorelist.Think, cfg.Ignorelist.Think) {
		t.Errorf("Think mismatch: got %v, want %v", loaded.Ignorelist.Think, cfg.Ignorelist.Think)
	}
}

func TestIgnorelist_MissingDefaultsToEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	minimal := []byte("server:\n  host: game.eternalcitygame.com\n  port: 8080\n  protocol: ws\n  login_url: https://login.eternalcitygame.com/login.php\n")
	if err := os.WriteFile(path, minimal, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Ignorelist.OOC) != 0 {
		t.Errorf("OOC should default empty, got %v", cfg.Ignorelist.OOC)
	}
	if len(cfg.Ignorelist.Think) != 0 {
		t.Errorf("Think should default empty, got %v", cfg.Ignorelist.Think)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
server:
  host: game.example.com
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("default Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Commands.DefaultDelay.String() != "1s" {
		t.Errorf("default DefaultDelay = %v, want 1s", cfg.Commands.DefaultDelay)
	}
	if cfg.UI.Scrollback != 5000 {
		t.Errorf("default Scrollback = %d, want 5000", cfg.UI.Scrollback)
	}
}

func TestSaveAndLoad_KudosRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	cfg := Defaults()
	cfg.Kudos.Favorites = []string{"Alice", "Bjorn"}
	cfg.Kudos.Queue = []KudosQueueEntry{
		{Name: "Cara", Message: "thanks for the rescue"},
		{Name: "Dren", Message: "great storytelling"},
	}

	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.Kudos.Favorites, []string{"Alice", "Bjorn"}) {
		t.Errorf("favorites round-trip mismatch: %v", got.Kudos.Favorites)
	}
	if !reflect.DeepEqual(got.Kudos.Queue, cfg.Kudos.Queue) {
		t.Errorf("queue round-trip mismatch: %v", got.Kudos.Queue)
	}
}

func TestLoad_KudosMissingSectionDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server:\n  host: example.com\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Kudos.Favorites) != 0 || len(got.Kudos.Queue) != 0 {
		t.Errorf("expected empty kudos, got %+v", got.Kudos)
	}
}

func TestKudosConfig_AddFavorite(t *testing.T) {
	cases := []struct {
		name      string
		initial   []string
		add       string
		want      []string
		wantAdded bool
	}{
		{"add to empty", nil, "Alice", []string{"Alice"}, true},
		{"sorted insert", []string{"Bjorn"}, "Alice", []string{"Alice", "Bjorn"}, true},
		{"trims whitespace", nil, "  Alice  ", []string{"Alice"}, true},
		{"case-insensitive dedup keeps original", []string{"Alice"}, "alice", []string{"Alice"}, false},
		{"case-insensitive sort", []string{"bob", "Alice"}, "Cara", []string{"Alice", "bob", "Cara"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := KudosConfig{Favorites: append([]string(nil), tc.initial...)}
			added := k.AddFavorite(tc.add)
			if added != tc.wantAdded {
				t.Errorf("AddFavorite added=%v, want %v", added, tc.wantAdded)
			}
			if !reflect.DeepEqual(k.Favorites, tc.want) {
				t.Errorf("favorites=%v, want %v", k.Favorites, tc.want)
			}
		})
	}
}

func TestKudosConfig_HasFavorite(t *testing.T) {
	k := KudosConfig{Favorites: []string{"Alice", "Bjorn"}}
	if !k.HasFavorite("alice") {
		t.Error("expected case-insensitive match for 'alice'")
	}
	if k.HasFavorite("Cara") {
		t.Error("did not expect match for 'Cara'")
	}
}

func TestKudosConfig_RemoveFavoriteAt(t *testing.T) {
	k := KudosConfig{Favorites: []string{"Alice", "Bjorn", "Cara"}}
	k.RemoveFavoriteAt(1)
	if !reflect.DeepEqual(k.Favorites, []string{"Alice", "Cara"}) {
		t.Errorf("after remove[1]: %v", k.Favorites)
	}
	k.RemoveFavoriteAt(99)
	if !reflect.DeepEqual(k.Favorites, []string{"Alice", "Cara"}) {
		t.Errorf("oob remove changed slice: %v", k.Favorites)
	}
}

func TestKudosConfig_QueueAddRemove(t *testing.T) {
	k := KudosConfig{}
	k.AddQueueEntry("Cara", "thanks")
	k.AddQueueEntry("  Cara  ", "  again  ")
	if len(k.Queue) != 2 {
		t.Fatalf("queue len=%d", len(k.Queue))
	}
	if k.Queue[1].Name != "Cara" || k.Queue[1].Message != "again" {
		t.Errorf("trim failure: %+v", k.Queue[1])
	}
	k.RemoveQueueAt(0)
	if len(k.Queue) != 1 || k.Queue[0].Message != "again" {
		t.Errorf("after remove: %+v", k.Queue)
	}
	k.RemoveQueueAt(99)
	if len(k.Queue) != 1 {
		t.Errorf("oob queue remove changed slice")
	}
}

func TestConfigActionSetsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Defaults()
	cfg.UI.ActionSets = []ActionSet{
		{Name: "Combat", Buttons: []ActionButton{
			{Label: "Attack", Command: "attack"},
			{Label: "Get all", Command: "get all"},
		}},
	}
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.UI.ActionSets) != 1 {
		t.Fatalf("ActionSets len = %d, want 1", len(got.UI.ActionSets))
	}
	set := got.UI.ActionSets[0]
	if set.Name != "Combat" {
		t.Errorf("Name = %q, want Combat", set.Name)
	}
	if len(set.Buttons) != 2 ||
		set.Buttons[0].Label != "Attack" || set.Buttons[0].Command != "attack" ||
		set.Buttons[1].Command != "get all" {
		t.Errorf("Buttons = %+v", set.Buttons)
	}
}

// TestSaveLoadListsRoundTrip guards cross-session persistence of the
// StringList-backed settings edited from the GUI/TUI menus: the ignore lists,
// high-priority commands, and script directories must survive Save -> Load
// (i.e. an app restart). Regression test for the GUI ignore lists not sticking.
func TestSaveLoadListsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	c := Defaults()
	c.Ignorelist.OOC = []string{"alice", "bob"}
	c.Ignorelist.Think = []string{"carol"}
	c.Commands.HighPriority = []string{"flee", "quaff"}
	c.Scripts = []string{"~/one/scripts", "~/two/scripts"}

	if err := Save(c, path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !reflect.DeepEqual(loaded.Ignorelist.OOC, c.Ignorelist.OOC) {
		t.Errorf("Ignorelist.OOC not persisted: got %v want %v", loaded.Ignorelist.OOC, c.Ignorelist.OOC)
	}
	if !reflect.DeepEqual(loaded.Ignorelist.Think, c.Ignorelist.Think) {
		t.Errorf("Ignorelist.Think not persisted: got %v want %v", loaded.Ignorelist.Think, c.Ignorelist.Think)
	}
	if !reflect.DeepEqual(loaded.Commands.HighPriority, c.Commands.HighPriority) {
		t.Errorf("Commands.HighPriority not persisted: got %v want %v", loaded.Commands.HighPriority, c.Commands.HighPriority)
	}
	if !reflect.DeepEqual(loaded.Scripts, c.Scripts) {
		t.Errorf("Scripts not persisted: got %v want %v", loaded.Scripts, c.Scripts)
	}
}

func TestValidate_NumpadNavigation(t *testing.T) {
	// Default is "numlock".
	if got := Defaults().UI.NumpadNavigation; got != "numlock" {
		t.Errorf("default NumpadNavigation = %q, want \"numlock\"", got)
	}

	// Empty (pre-existing configs) and unrecognized values normalize to "numlock".
	for _, in := range []string{"", "bogus", "NumLock"} {
		cfg := Defaults()
		cfg.UI.NumpadNavigation = in
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error: %v", err)
		}
		if cfg.UI.NumpadNavigation != "numlock" {
			t.Errorf("NumpadNavigation %q normalized to %q, want \"numlock\"", in, cfg.UI.NumpadNavigation)
		}
	}

	// Valid values are preserved.
	for _, in := range []string{"numlock", "always", "off"} {
		cfg := Defaults()
		cfg.UI.NumpadNavigation = in
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error: %v", err)
		}
		if cfg.UI.NumpadNavigation != in {
			t.Errorf("NumpadNavigation %q changed to %q, want preserved", in, cfg.UI.NumpadNavigation)
		}
	}
}

func TestLoadConfig_SpellcheckAndUpdatesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	// Neither key present: both default on (pre-existing configs get the
	// features without editing their YAML).
	if err := os.WriteFile(cfgPath, []byte("server:\n  host: example.com\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got.UI.InputSpellcheck {
		t.Error("ui.input_spellcheck should default true when absent")
	}
	if !got.Updates.Check {
		t.Error("updates.check should default true when absent")
	}
}

func TestLoadConfig_SpellcheckAndUpdatesExplicitOff(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	yaml := "ui:\n  input_spellcheck: false\nupdates:\n  check: false\n"
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.UI.InputSpellcheck {
		t.Error("ui.input_spellcheck: false should be honored")
	}
	if got.Updates.Check {
		t.Error("updates.check: false should be honored")
	}
}
