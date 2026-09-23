package client

import (
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/types"
)

func TestLuaNotifyReachesClientEvents(t *testing.T) {
	c, err := NewClient(config.Defaults(), nil, t.TempDir(), nil)
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
