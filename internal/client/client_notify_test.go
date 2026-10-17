package client

import (
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestDesktopNotificationCommandSoundPolicy(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		sound     bool
		want      string
		doNotWant string
	}{
		{name: "linux sound", goos: "linux", sound: true, want: "string:sound-name:message-new-instant", doNotWant: "suppress-sound"},
		{name: "linux silent", goos: "linux", sound: false, want: "boolean:suppress-sound:true", doNotWant: "sound-name"},
		{name: "macOS sound", goos: "darwin", sound: true, want: `sound name "default"`},
		{name: "macOS silent", goos: "darwin", sound: false, doNotWant: "sound name"},
		{name: "Windows sound", goos: "windows", sound: true, want: "ms-winsoundevent:Notification.Default", doNotWant: "silent', 'true"},
		{name: "Windows silent", goos: "windows", sound: false, want: "silent', 'true", doNotWant: "ms-winsoundevent:Notification.Default"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := desktopNotificationCommand(tc.goos, "Title", "Message", tc.sound)
			if cmd == nil {
				t.Fatal("expected a notification command")
			}
			args := strings.Join(cmd.Args, " ")
			if tc.want != "" && !strings.Contains(args, tc.want) {
				t.Errorf("command %q does not contain %q", args, tc.want)
			}
			if tc.doNotWant != "" && strings.Contains(args, tc.doNotWant) {
				t.Errorf("command %q unexpectedly contains %q", args, tc.doNotWant)
			}
		})
	}

	if cmd := desktopNotificationCommand("plan9", "Title", "Message", true); cmd != nil {
		t.Fatalf("unsupported OS returned command: %v", cmd.Args)
	}
}

func TestUpdateConfig_RecompilesPatterns(t *testing.T) {
	// Start with one pattern.
	initial := config.DesktopNotificationsConfig{
		Patterns: []config.NotifyPatternConfig{
			{Pattern: "hello*", Title: "Hello", Message: "Got hello", Enabled: true},
		},
	}
	dn := NewDesktopNotifier(initial)

	if len(dn.patterns) != 1 {
		t.Fatalf("expected 1 initial pattern, got %d", len(dn.patterns))
	}
	if dn.patterns[0].pattern != "hello*" {
		t.Errorf("expected pattern 'hello*', got %q", dn.patterns[0].pattern)
	}

	// Update with two different patterns.
	updated := config.DesktopNotificationsConfig{
		Sound:       true,
		HealthBelow: config.ThresholdConfig{Enabled: true, Threshold: 15},
		Patterns: []config.NotifyPatternConfig{
			{Pattern: "attack*", Title: "Combat", Message: "Combat started", Enabled: true},
			{Pattern: "you ?die", Title: "Death", Message: "You died", Enabled: false},
		},
	}
	dn.UpdateConfig(updated)

	// Verify cfg updated.
	if !dn.cfg.HealthBelow.Enabled {
		t.Error("expected HealthBelow.Enabled=true after UpdateConfig")
	}
	if dn.cfg.HealthBelow.Threshold != 15 {
		t.Errorf("expected HealthBelow.Threshold=15, got %d", dn.cfg.HealthBelow.Threshold)
	}
	if !dn.cfg.Sound {
		t.Error("expected Sound=true after UpdateConfig")
	}

	// Verify patterns recompiled.
	if len(dn.patterns) != 2 {
		t.Fatalf("expected 2 patterns after UpdateConfig, got %d", len(dn.patterns))
	}
	if dn.patterns[0].pattern != "attack*" {
		t.Errorf("expected first pattern 'attack*', got %q", dn.patterns[0].pattern)
	}
	if dn.patterns[1].pattern != "you ?die" {
		t.Errorf("expected second pattern 'you ?die', got %q", dn.patterns[1].pattern)
	}
	if dn.patterns[1].enabled {
		t.Error("expected second pattern disabled")
	}

	// Raw rules are retained so variables can be resolved against the current
	// snapshot at match time rather than frozen during configuration updates.
}

func TestUpdateConfig_PreservesDedup(t *testing.T) {
	initial := config.DesktopNotificationsConfig{
		HealthBelow: config.ThresholdConfig{Enabled: true, Threshold: 25},
	}
	dn := NewDesktopNotifier(initial)

	// Populate lastSent by calling send directly.
	dn.send("Test", "test message", "health")

	// Verify lastSent is populated.
	dn.mu.Lock()
	if len(dn.lastSent) != 1 {
		t.Fatalf("expected 1 lastSent entry, got %d", len(dn.lastSent))
	}
	sentTime := dn.lastSent["health"]
	dn.mu.Unlock()

	if sentTime.IsZero() {
		t.Fatal("expected non-zero lastSent time for 'health'")
	}

	// Sleep briefly so we can detect if time was reset.
	time.Sleep(5 * time.Millisecond)

	// Update config.
	updated := config.DesktopNotificationsConfig{
		HealthBelow: config.ThresholdConfig{Enabled: true, Threshold: 10},
	}
	dn.UpdateConfig(updated)

	// Verify lastSent is preserved (same entry, same time).
	dn.mu.Lock()
	defer dn.mu.Unlock()

	if len(dn.lastSent) != 1 {
		t.Fatalf("expected lastSent preserved with 1 entry, got %d", len(dn.lastSent))
	}
	if dn.lastSent["health"] != sentTime {
		t.Errorf("expected lastSent time preserved, got %v (was %v)", dn.lastSent["health"], sentTime)
	}
}

func TestDesktopNotifier_UsesInjectedSink(t *testing.T) {
	cfg := config.DesktopNotificationsConfig{
		HealthBelow: config.ThresholdConfig{Enabled: true, Threshold: 25},
	}
	dn := NewDesktopNotifier(cfg)
	type delivered struct{ title, message string }
	got := make(chan delivered, 1)
	dn.SetSink(NotificationSinkFunc(func(title, message string) {
		got <- delivered{title: title, message: message}
	}))

	dn.CheckHealth(20)
	select {
	case notification := <-got:
		if notification.title != "Health Warning" || notification.message != "Health at 20%" {
			t.Fatalf("unexpected notification: %#v", notification)
		}
	case <-time.After(time.Second):
		t.Fatal("injected notification sink was not called")
	}
}

func TestDesktopNotifier_NilSinkDoesNotDeliverNatively(t *testing.T) {
	dn := NewDesktopNotifier(config.DesktopNotificationsConfig{
		HealthBelow: config.ThresholdConfig{Enabled: true, Threshold: 25},
	})
	dn.SetSink(nil)
	// The assertion is that this returns without attempting to dereference the
	// sink. Native process execution is deliberately outside unit tests.
	dn.CheckHealth(20)
}

func TestDesktopNotifier_ExpandsLiveVariablesInRulesAndTemplates(t *testing.T) {
	dn := NewDesktopNotifier(config.DesktopNotificationsConfig{Patterns: []config.NotifyPatternConfig{{
		Pattern: `${target} dropped \*`,
		Title:   "Loot: ${target}",
		Message: "Owner ${owner:unknown}",
		Enabled: true,
	}}})
	type delivered struct{ title, message string }
	got := make(chan delivered, 2)
	dn.SetSink(NotificationSinkFunc(func(title, message string) {
		got <- delivered{title: title, message: message}
	}))
	dn.SetVariables(map[string]string{"target": "Apula"})

	dn.CheckText("Apula dropped *")
	select {
	case notification := <-got:
		if notification.title != "Loot: Apula" || notification.message != "Owner unknown" {
			t.Fatalf("unexpected expanded notification: %#v", notification)
		}
	case <-time.After(time.Second):
		t.Fatal("expanded notification was not delivered")
	}

	dn.SetVariables(map[string]string{"target": "Constantine", "owner": "Neil"})
	dn.CheckText("Apula dropped *")
	select {
	case notification := <-got:
		t.Fatalf("stale variable snapshot matched: %#v", notification)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestDesktopNotifier_UnresolvedPatternVariableMatchesNothing(t *testing.T) {
	dn := NewDesktopNotifier(config.DesktopNotificationsConfig{Patterns: []config.NotifyPatternConfig{{
		Pattern: "${missing}", Enabled: true,
	}}})
	got := make(chan struct{}, 1)
	dn.SetSink(NotificationSinkFunc(func(string, string) { got <- struct{}{} }))
	dn.CheckText("missing")
	select {
	case <-got:
		t.Fatal("unresolved notification pattern matched")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestDesktopNotifier_UnresolvedTemplateSuppressesAndWarnsOnce(t *testing.T) {
	dn := NewDesktopNotifier(config.DesktopNotificationsConfig{Patterns: []config.NotifyPatternConfig{{
		Pattern: "loot", Title: "${missing}", Enabled: true,
	}}})
	delivered := 0
	dn.SetSink(NotificationSinkFunc(func(string, string) { delivered++ }))
	warnings := make(chan string, 2)
	dn.SetWarningSink(func(message string) { warnings <- message })

	dn.CheckText("loot")
	dn.CheckText("loot again")
	if delivered != 0 {
		t.Fatalf("unresolved template delivered %d notifications", delivered)
	}
	select {
	case warning := <-warnings:
		if !strings.Contains(warning, "missing") {
			t.Fatalf("warning %q does not identify the unresolved variable", warning)
		}
	case <-time.After(time.Second):
		t.Fatal("missing in-app warning")
	}
	select {
	case warning := <-warnings:
		t.Fatalf("duplicate warning was not rate limited: %q", warning)
	case <-time.After(20 * time.Millisecond):
	}
}
