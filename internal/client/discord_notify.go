package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cyber-godzilla/praetor/internal/config"
)

type NotificationCategory string

const (
	NotificationCategoryHealth  NotificationCategory = "health"
	NotificationCategoryFatigue NotificationCategory = "fatigue"
	NotificationCategoryPattern NotificationCategory = "pattern"
	NotificationCategoryScript  NotificationCategory = "script"
)

type discordWebhookPayload struct {
	Content         string                 `json:"content"`
	AllowedMentions discordAllowedMentions `json:"allowed_mentions"`
}

type discordAllowedMentions struct {
	Parse []string `json:"parse"`
}

// DiscordNotifier delivers selected notifications to one Discord webhook.
// It owns a bounded HTTP client so delivery can never stall the game loop.
type DiscordNotifier struct {
	mu           sync.RWMutex
	cfg          config.DiscordNotificationsConfig
	httpClient   *http.Client
	allowTestURL bool
}

func NewDiscordNotifier(cfg config.DiscordNotificationsConfig) *DiscordNotifier {
	return &DiscordNotifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if err := validateDiscordWebhookURL(req.URL.String(), false); err != nil {
					return fmt.Errorf("Discord webhook redirect rejected")
				}
				return nil
			},
		},
	}
}

func (n *DiscordNotifier) UpdateConfig(cfg config.DiscordNotificationsConfig) {
	n.mu.Lock()
	n.cfg = cfg
	n.mu.Unlock()
}

func (n *DiscordNotifier) Config() config.DiscordNotificationsConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.cfg
}

func (n *DiscordNotifier) Send(ctx context.Context, category NotificationCategory, title, message string) error {
	n.mu.RLock()
	cfg := n.cfg
	client := n.httpClient
	allowTestURL := n.allowTestURL
	n.mu.RUnlock()
	if !cfg.Enabled || !discordCategoryEnabled(cfg, category) {
		return nil
	}
	return sendDiscordWebhook(ctx, client, cfg.WebhookURL, title, message, allowTestURL)
}

// SendTest verifies the configured webhook even when Discord delivery or all
// individual categories are disabled.
func (n *DiscordNotifier) SendTest(ctx context.Context) error {
	n.mu.RLock()
	cfg := n.cfg
	client := n.httpClient
	allowTestURL := n.allowTestURL
	n.mu.RUnlock()
	return sendDiscordWebhook(ctx, client, cfg.WebhookURL, "Praetor test", "Discord notifications are configured correctly.", allowTestURL)
}

func discordCategoryEnabled(cfg config.DiscordNotificationsConfig, category NotificationCategory) bool {
	switch category {
	case NotificationCategoryHealth:
		return cfg.Health
	case NotificationCategoryFatigue:
		return cfg.Fatigue
	case NotificationCategoryPattern:
		return cfg.Patterns
	case NotificationCategoryScript:
		return cfg.Scripts
	default:
		return false
	}
}

func sendDiscordWebhook(ctx context.Context, client *http.Client, rawURL, title, message string, allowTestURL bool) error {
	if err := validateDiscordWebhookURL(rawURL, allowTestURL); err != nil {
		return err
	}
	content := strings.TrimSpace("**" + title + "**\n" + message)
	content = truncateUTF8(content, 2000)
	payload, err := json.Marshal(discordWebhookPayload{
		Content:         content,
		AllowedMentions: discordAllowedMentions{Parse: []string{}},
	})
	if err != nil {
		return fmt.Errorf("encode Discord notification: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create Discord notification request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Praetor")
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("Discord notification timed out")
		}
		// net/http errors commonly embed the complete request URL. Never wrap or
		// format one here because the URL contains the webhook credential.
		return fmt.Errorf("deliver Discord notification: request failed")
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Discord notification rejected with HTTP %d", response.StatusCode)
	}
	return nil
}

func validateDiscordWebhookURL(rawURL string, allowTestURL bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid Discord webhook URL")
	}
	if allowTestURL {
		if parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("Discord webhook URL must use HTTPS")
		}
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" || (host != "discord.com" && host != "discordapp.com" && host != "canary.discord.com" && host != "ptb.discord.com") {
		return fmt.Errorf("Discord webhook URL must be an official HTTPS Discord URL")
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "webhooks" || parts[2] == "" || parts[3] == "" {
		return fmt.Errorf("invalid Discord webhook URL")
	}
	return nil
}

// ValidateDiscordWebhookURL validates a user-supplied webhook without making a
// network request. The returned error never contains the supplied URL.
func ValidateDiscordWebhookURL(rawURL string) error {
	return validateDiscordWebhookURL(strings.TrimSpace(rawURL), false)
}

func truncateUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
