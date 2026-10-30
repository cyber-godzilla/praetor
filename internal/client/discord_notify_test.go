package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
)

func TestDiscordNotifierRoutesEnabledCategories(t *testing.T) {
	requests := make(chan discordWebhookPayload, 1)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload discordWebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		requests <- payload
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	n := NewDiscordNotifier(config.DiscordNotificationsConfig{
		Enabled: true, WebhookURL: "https://discord.com/api/webhooks/123/token", Health: true,
	})
	n.httpClient = &http.Client{Transport: transport}

	if err := n.Send(context.Background(), NotificationCategoryHealth, "Health Warning", "Health at 20%"); err != nil {
		t.Fatalf("Send health: %v", err)
	}
	select {
	case payload := <-requests:
		if payload.Content != "**Health Warning**\nHealth at 20%" {
			t.Fatalf("content = %q", payload.Content)
		}
		if payload.AllowedMentions.Parse == nil || len(payload.AllowedMentions.Parse) != 0 {
			t.Fatalf("allowed mentions = %#v, want explicit empty parse list", payload.AllowedMentions.Parse)
		}
	case <-time.After(time.Second):
		t.Fatal("health notification was not delivered")
	}

	if err := n.Send(context.Background(), NotificationCategoryPattern, "Alert", "loot"); err != nil {
		t.Fatalf("disabled category returned error: %v", err)
	}
	select {
	case <-requests:
		t.Fatal("disabled pattern category was delivered")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestDiscordNotifierRequiresOfficialHTTPSWebhook(t *testing.T) {
	tests := []string{
		"http://discord.com/api/webhooks/123/token",
		"https://example.com/api/webhooks/123/token",
		"https://discord.com/not-a-webhook",
		"https://discord.com/api/webhooks/123",
	}
	for _, rawURL := range tests {
		n := NewDiscordNotifier(config.DiscordNotificationsConfig{Enabled: true, WebhookURL: rawURL, Health: true})
		if err := n.Send(context.Background(), NotificationCategoryHealth, "Title", "Message"); err == nil {
			t.Errorf("Send(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestDiscordNotifierErrorsDoNotExposeWebhook(t *testing.T) {
	secret := "very-secret-token"
	n := NewDiscordNotifier(config.DiscordNotificationsConfig{
		Enabled:    true,
		WebhookURL: "https://discord.com/api/webhooks/123/" + secret,
		Health:     true,
	})
	n.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})}
	err := n.Send(context.Background(), NotificationCategoryHealth, "Title", "Message")
	if err == nil {
		t.Fatal("expected delivery error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("delivery error exposed webhook token: %v", err)
	}
}

func TestDiscordNotifierTestBypassesDestinationSwitches(t *testing.T) {
	received := make(chan struct{}, 1)
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		received <- struct{}{}
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})
	n := NewDiscordNotifier(config.DiscordNotificationsConfig{WebhookURL: "https://discord.com/api/webhooks/123/token"})
	n.httpClient = &http.Client{Transport: transport}
	if err := n.SendTest(context.Background()); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("test notification was not delivered")
	}
}

func TestNotificationCategoryForKey(t *testing.T) {
	tests := map[string]NotificationCategory{
		"health":            NotificationCategoryHealth,
		"fatigue":           NotificationCategoryFatigue,
		"pattern:rare drop": NotificationCategoryPattern,
		"explicit:Lua":      NotificationCategoryScript,
	}
	for key, want := range tests {
		if got := notificationCategoryForKey(key); got != want {
			t.Errorf("notificationCategoryForKey(%q) = %q, want %q", key, got, want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
