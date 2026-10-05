package client

import (
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestLuaNotifyReachesClientEvents(t *testing.T) {
	cfg := config.Defaults()
	cfg.Notifications.Desktop.AllowScriptNotifications = true
	c, err := NewClient(cfg, nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Engine.Close()
	c.desktopNotify = func(string, string, bool) {}

	c.Engine.OnNotify("Alert", "Incoming attack")

	select {
	case event := <-c.Events():
		notification, ok := event.(types.NotificationEvent)
		if !ok {
			t.Fatalf("event type = %T, want NotificationEvent", event)
		}
		if notification.Title != "Alert" || notification.Message != "Incoming attack" {
			t.Fatalf("notification = %#v", notification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Lua notification event")
	}
}

func TestLuaNotifyIsSuppressedWhenScriptNotificationsAreDisabled(t *testing.T) {
	c, err := NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Engine.Close()

	desktopCalled := make(chan struct{}, 1)
	c.desktopNotify = func(string, string, bool) { desktopCalled <- struct{}{} }
	c.Engine.OnNotify("Alert", "Incoming attack")

	select {
	case event := <-c.Events():
		t.Fatalf("disabled script notification emitted %T", event)
	case <-desktopCalled:
		t.Fatal("disabled script notification reached the desktop notifier")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestScriptNotificationPreferencesApplyLive(t *testing.T) {
	c, err := NewClient(config.Defaults(), nil, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Engine.Close()

	desktopSound := make(chan bool, 1)
	c.desktopNotify = func(_ string, _ string, sound bool) { desktopSound <- sound }
	c.SetScriptNotificationPreferences(true, true)
	c.Engine.OnNotify("Alert", "Incoming attack")

	select {
	case sound := <-desktopSound:
		if !sound {
			t.Fatal("live-enabled script notification did not use the updated sound setting")
		}
	case <-time.After(time.Second):
		t.Fatal("live-enabled script notification did not reach the desktop notifier")
	}
}
