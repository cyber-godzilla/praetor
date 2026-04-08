package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if cfg.Reconnect.MaxDelay.String() != "30s" {
		t.Errorf("Reconnect.MaxDelay = %v, want 30s", cfg.Reconnect.MaxDelay)
	}
	if cfg.Commands.DefaultDelay.String() != "800ms" {
		t.Errorf("Commands.DefaultDelay = %v, want 800ms", cfg.Commands.DefaultDelay)
	}
	if cfg.UI.SidebarOpen != false {
		t.Error("UI.SidebarOpen should be false")
	}
	if len(cfg.Commands.HighPriority) != 2 {
		t.Errorf("HighPriority len = %d, want 2", len(cfg.Commands.HighPriority))
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
	cfg.Reconnect.BackoffMultiplier = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if cfg.UI.Scrollback != 5000 {
		t.Errorf("Scrollback = %d, want 5000", cfg.UI.Scrollback)
	}
	if cfg.UI.SidebarWidth != 40 {
		t.Errorf("SidebarWidth = %d, want 40", cfg.UI.SidebarWidth)
	}
	if cfg.UI.MinimapScale != 0.8 {
		t.Errorf("MinimapScale = %f, want 0.8", cfg.UI.MinimapScale)
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
	if cfg.Reconnect.BackoffMultiplier != 2 {
		t.Errorf("BackoffMultiplier = %d, want 2", cfg.Reconnect.BackoffMultiplier)
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
	if cfg.UI.MinimapScale != 0.8 {
		t.Errorf("default MinimapScale = %f, want 0.8", cfg.UI.MinimapScale)
	}
	if cfg.UI.MinimapHeight != 12 {
		t.Errorf("default MinimapHeight = %d, want 12", cfg.UI.MinimapHeight)
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
