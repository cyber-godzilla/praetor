package client

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
	sharedvars "github.com/cyber-godzilla/praetor/internal/variables"
	"github.com/cyber-godzilla/praetor/internal/wildcard"
)

// DesktopNotifier sends desktop notifications for health, fatigue,
// and pattern-matched game text.
type DesktopNotifier struct {
	mu          sync.RWMutex
	cfg         config.DesktopNotificationsConfig
	patterns    []*compiledNotifyPattern
	lastSent    map[string]time.Time // dedup key → last send time
	cooldown    time.Duration
	sink        NotificationSink
	variables   map[string]string
	warningSink func(string)
	warningLast map[string]time.Time
	discord     *DiscordNotifier
	focused     bool
}

// SetDiscordNotifier adds the optional remote delivery destination. Matching
// and cooldowns remain owned by DesktopNotifier so both destinations observe
// exactly the same rule decisions.
func (dn *DesktopNotifier) SetDiscordNotifier(notifier *DiscordNotifier) {
	dn.mu.Lock()
	dn.discord = notifier
	dn.mu.Unlock()
}

// NotificationSink receives notifications after rule evaluation and cooldown
// handling. Native shells use NativeNotificationSink; the headless web shell
// installs a projection sink so it never launches desktop helper processes on
// the server host.
type NotificationSink interface {
	Notify(title, message string)
}

type NotificationSinkFunc func(title, message string)

func (f NotificationSinkFunc) Notify(title, message string) { f(title, message) }

// NotificationOptions carries the per-rule presentation choices to shells
// that can honor them locally (notably each praetor-web browser).
type NotificationOptions struct {
	Sound             bool
	OnlyWhenUnfocused bool
}

type NotificationOptionsSink interface {
	NotifyWithOptions(title, message string, options NotificationOptions)
}

type NotificationOptionsSinkFunc func(title, message string, options NotificationOptions)

func (f NotificationOptionsSinkFunc) Notify(title, message string) {
	f(title, message, NotificationOptions{})
}
func (f NotificationOptionsSinkFunc) NotifyWithOptions(title, message string, options NotificationOptions) {
	f(title, message, options)
}

type NativeNotificationSink struct {
	Sound bool
}

func (s NativeNotificationSink) Notify(title, message string) {
	go sendDesktopNotification(title, message, s.Sound)
}

type compiledNotifyPattern struct {
	pattern  string
	title    string
	message  string
	enabled  bool
	when     string
	sound    bool
	delivery string
}

// NewDesktopNotifier creates a notifier from config.
func NewDesktopNotifier(cfg config.DesktopNotificationsConfig) *DesktopNotifier {
	dn := &DesktopNotifier{
		cfg:         cfg,
		lastSent:    make(map[string]time.Time),
		warningLast: make(map[string]time.Time),
		cooldown:    30 * time.Second,
		sink:        NativeNotificationSink{Sound: cfg.Sound},
	}

	for _, p := range cfg.Patterns {
		dn.patterns = append(dn.patterns, &compiledNotifyPattern{
			pattern: p.Pattern,
			title:   p.Title,
			message: p.Message,
			enabled: p.Enabled,
			when:    p.When, sound: p.Sound, delivery: p.Delivery,
		})
	}

	return dn
}

// SetFocused lets the native shell suppress rules configured for background
// delivery. Web browsers apply the same option independently in each tab.
func (dn *DesktopNotifier) SetFocused(focused bool) {
	dn.mu.Lock()
	dn.focused = focused
	dn.mu.Unlock()
}

// SetWarningSink installs the in-app destination for rule diagnostics.
func (dn *DesktopNotifier) SetWarningSink(sink func(string)) {
	dn.mu.Lock()
	dn.warningSink = sink
	dn.mu.Unlock()
}

// SetVariables replaces the snapshot used to resolve notification patterns
// and output templates at match/delivery time.
func (dn *DesktopNotifier) SetVariables(variables map[string]string) {
	cloned := make(map[string]string, len(variables))
	for name, value := range variables {
		cloned[name] = value
	}
	dn.mu.Lock()
	dn.variables = cloned
	dn.mu.Unlock()
}

// SetSink changes notification delivery without changing matching rules or
// cooldown state. A nil sink intentionally discards native delivery.
func (dn *DesktopNotifier) SetSink(sink NotificationSink) {
	dn.mu.Lock()
	dn.sink = sink
	dn.mu.Unlock()
}

// UpdateConfig replaces the notifier's configuration and recompiles patterns.
// The dedup map (lastSent) is preserved so cooldowns survive config reloads.
func (dn *DesktopNotifier) UpdateConfig(cfg config.DesktopNotificationsConfig) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	dn.cfg = cfg
	if _, native := dn.sink.(NativeNotificationSink); native {
		dn.sink = NativeNotificationSink{Sound: cfg.Sound}
	}

	dn.patterns = nil
	for _, p := range cfg.Patterns {
		dn.patterns = append(dn.patterns, &compiledNotifyPattern{
			pattern: p.Pattern,
			title:   p.Title,
			message: p.Message,
			enabled: p.Enabled,
			when:    p.When, sound: p.Sound, delivery: p.Delivery,
		})
	}
}

// CheckHealth sends a desktop notification if health drops below threshold.
func (dn *DesktopNotifier) CheckHealth(health int) {
	dn.mu.RLock()
	rule := dn.cfg.HealthBelow
	dn.mu.RUnlock()

	if !rule.Enabled {
		return
	}
	if health <= rule.Threshold {
		dn.sendRule("Health Warning", fmt.Sprintf("Health at %d%%", health), "health", rule.When, rule.Sound, rule.Delivery)
	}
}

// CheckFatigue sends a desktop notification if fatigue drops below threshold.
func (dn *DesktopNotifier) CheckFatigue(fatigue int) {
	dn.mu.RLock()
	rule := dn.cfg.FatigueBelow
	dn.mu.RUnlock()

	if !rule.Enabled {
		return
	}
	if fatigue <= rule.Threshold {
		dn.sendRule("Fatigue Warning", fmt.Sprintf("Fatigue at %d%%", fatigue), "fatigue", rule.When, rule.Sound, rule.Delivery)
	}
}

// CheckText sends a desktop notification if text matches any enabled pattern.
func (dn *DesktopNotifier) CheckText(text string) {
	dn.mu.RLock()
	patterns := append([]*compiledNotifyPattern(nil), dn.patterns...)
	variables := cloneVariables(dn.variables)
	dn.mu.RUnlock()

	for index, p := range patterns {
		if !p.enabled {
			continue
		}
		expandedPattern, err := sharedvars.Expand(strings.TrimSpace(p.pattern), variables)
		if err != nil || expandedPattern == "" {
			continue
		}
		matcher, err := wildcard.Compile(expandedPattern, wildcard.Substring)
		if err != nil || !matcher.Match(text) {
			continue
		}
		title, err := sharedvars.Expand(p.title, variables)
		if err != nil {
			dn.warn(fmt.Sprintf("pattern:%d:%s:title", index, p.pattern), fmt.Sprintf("Notification rule %q title: %v", p.pattern, err))
			continue
		}
		msg, err := sharedvars.Expand(p.message, variables)
		if err != nil {
			dn.warn(fmt.Sprintf("pattern:%d:%s:message", index, p.pattern), fmt.Sprintf("Notification rule %q message: %v", p.pattern, err))
			continue
		}
		if title == "" {
			title = "Alert"
		}
		if msg == "" {
			msg = text
		}
		dn.sendRule(title, msg, "pattern:"+p.pattern, p.when, p.sound, p.delivery)
		return // one notification per text line
	}
}

// Notify delivers an explicit Lua/application notification through the active
// shell sink, with the same cooldown protection as rule-generated alerts.
func (dn *DesktopNotifier) Notify(title, message string) {
	dn.send(title, message, "explicit:"+title+"\x00"+message)
}

// Prune removes stale dedup entries older than the cooldown window.
func (dn *DesktopNotifier) Prune() {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	now := time.Now()
	for key, last := range dn.lastSent {
		if now.Sub(last) >= dn.cooldown {
			delete(dn.lastSent, key)
		}
	}
	for key, last := range dn.warningLast {
		if now.Sub(last) >= dn.cooldown {
			delete(dn.warningLast, key)
		}
	}
}

func (dn *DesktopNotifier) warn(key, message string) {
	dn.mu.Lock()
	now := time.Now()
	if last, ok := dn.warningLast[key]; ok && now.Sub(last) < dn.cooldown {
		dn.mu.Unlock()
		return
	}
	dn.warningLast[key] = now
	sink := dn.warningSink
	dn.mu.Unlock()
	if sink != nil {
		sink(message)
	}
}

// send dispatches a desktop notification with rate limiting.
func (dn *DesktopNotifier) send(title, message, dedupKey string) {
	dn.mu.RLock()
	when, sound, delivery := config.NotificationWhenAlways, dn.cfg.Sound, config.NotificationDeliveryDesktop
	switch notificationCategoryForKey(dedupKey) {
	case NotificationCategoryHealth:
		when, sound, delivery = dn.cfg.HealthBelow.When, dn.cfg.HealthBelow.Sound, dn.cfg.HealthBelow.Delivery
	case NotificationCategoryFatigue:
		when, sound, delivery = dn.cfg.FatigueBelow.When, dn.cfg.FatigueBelow.Sound, dn.cfg.FatigueBelow.Delivery
	}
	dn.mu.RUnlock()
	dn.sendRule(title, message, dedupKey, when, sound, delivery)
}

func (dn *DesktopNotifier) sendRule(title, message, dedupKey, when string, sound bool, delivery string) {
	dn.mu.Lock()
	if when != config.NotificationWhenUnfocused {
		when = config.NotificationWhenAlways
	}
	if delivery == "" {
		delivery = config.NotificationDeliveryDesktop
	}
	desktop := delivery == config.NotificationDeliveryDesktop || delivery == config.NotificationDeliveryBoth
	discordDelivery := delivery == config.NotificationDeliveryDiscord || delivery == config.NotificationDeliveryBoth
	if when == config.NotificationWhenUnfocused && dn.focused {
		desktop = false
	}
	if !desktop && !discordDelivery {
		dn.mu.Unlock()
		return
	}

	now := time.Now()
	if last, ok := dn.lastSent[dedupKey]; ok {
		if now.Sub(last) < dn.cooldown {
			dn.mu.Unlock()
			return
		}
	}
	dn.lastSent[dedupKey] = now
	sink := dn.sink
	discord := dn.discord
	dn.mu.Unlock()

	options := NotificationOptions{Sound: sound, OnlyWhenUnfocused: when == config.NotificationWhenUnfocused}
	if desktop && sink != nil {
		if configured, ok := sink.(NotificationOptionsSink); ok {
			configured.NotifyWithOptions(title, message, options)
		} else if _, native := sink.(NativeNotificationSink); native {
			go sendDesktopNotification(title, message, sound)
		} else {
			sink.Notify(title, message)
		}
	}
	if discordDelivery && discord != nil {
		category := notificationCategoryForKey(dedupKey)
		go func() {
			if err := discord.Send(context.Background(), category, title, message); err != nil {
				log.Printf("[NOTIFY] Discord delivery failed: %v", err)
			}
		}()
	}
}

func notificationCategoryForKey(key string) NotificationCategory {
	switch {
	case key == "health":
		return NotificationCategoryHealth
	case key == "fatigue":
		return NotificationCategoryFatigue
	case strings.HasPrefix(key, "pattern:"):
		return NotificationCategoryPattern
	default:
		return NotificationCategoryScript
	}
}

// sendDesktopNotification sends a notification using the platform's native mechanism.
// All arguments are passed as separate command args to avoid shell injection.
func sendDesktopNotification(title, message string, sound bool) {
	cmd := desktopNotificationCommand(runtime.GOOS, title, message, sound)
	if cmd == nil {
		log.Printf("[NOTIFY] desktop notifications not supported on %s", runtime.GOOS)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[NOTIFY] desktop notification failed: %v", err)
	}
	// Reap the child process to avoid zombies (#4).
	go cmd.Wait()
}

// desktopNotificationCommand builds the native notification command for goos.
// sound selects the OS-managed default alert sound; false explicitly suppresses
// sound where the platform notification API supports doing so.
func desktopNotificationCommand(goos, title, message string, sound bool) *exec.Cmd {
	switch goos {
	case "linux":
		// notify-send takes title and message as separate args — no shell.
		hint := "boolean:suppress-sound:true"
		if sound {
			hint = "string:sound-name:message-new-instant"
		}
		return exec.Command("notify-send", "--hint="+hint, title, message)
	case "darwin":
		// Pass title and message via -e args to avoid shell injection.
		// osascript receives pre-escaped strings via separate arguments.
		sanitize := func(s string) string {
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", "")
			s = strings.ReplaceAll(s, "\t", " ")
			return s
		}
		sanitizedTitle := sanitize(title)
		sanitizedMsg := sanitize(message)
		script := `display notification "` + sanitizedMsg + `" with title "` + sanitizedTitle + `"`
		if sound {
			script += ` sound name "default"`
		}
		return exec.Command("osascript", "-e", script)
	case "windows":
		// Use PowerShell with escaped single quotes.
		escapedTitle := strings.ReplaceAll(title, "'", "''")
		escapedMsg := strings.ReplaceAll(message, "'", "''")
		audioValue := "ms-winsoundevent:Notification.Default"
		audioAttribute := "src"
		if !sound {
			audioValue = "true"
			audioAttribute = "silent"
		}
		ps := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; ` +
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(0); ` +
			`$text = $template.GetElementsByTagName('text'); ` +
			`$text.Item(0).AppendChild($template.CreateTextNode('` + escapedTitle + `')) | Out-Null; ` +
			`$text.Item(1).AppendChild($template.CreateTextNode('` + escapedMsg + `')) | Out-Null; ` +
			`$audio = $template.CreateElement('audio'); ` +
			`$audio.SetAttribute('` + audioAttribute + `', '` + audioValue + `'); ` +
			`$template.DocumentElement.AppendChild($audio) | Out-Null; ` +
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); ` +
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Praetor').Show($toast)`
		return exec.Command("powershell", "-Command", ps)
	default:
		return nil
	}
}
