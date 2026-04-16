package client

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/config"
)

// DesktopNotifier sends desktop notifications for health, fatigue,
// and pattern-matched game text.
type DesktopNotifier struct {
	mu       sync.Mutex
	cfg      config.DesktopNotificationsConfig
	patterns []*compiledNotifyPattern
	lastSent map[string]time.Time // dedup key → last send time
	cooldown time.Duration
}

type compiledNotifyPattern struct {
	pattern string
	title   string
	message string
	regex   *regexp.Regexp
	enabled bool
}

// NewDesktopNotifier creates a notifier from config.
func NewDesktopNotifier(cfg config.DesktopNotificationsConfig) *DesktopNotifier {
	dn := &DesktopNotifier{
		cfg:      cfg,
		lastSent: make(map[string]time.Time),
		cooldown: 30 * time.Second,
	}

	for _, p := range cfg.Patterns {
		escaped := regexp.QuoteMeta(p.Pattern)
		escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
		escaped = strings.ReplaceAll(escaped, `\?`, `.`)
		re, err := regexp.Compile("(?i)" + escaped)
		if err != nil {
			continue
		}
		dn.patterns = append(dn.patterns, &compiledNotifyPattern{
			pattern: p.Pattern,
			title:   p.Title,
			message: p.Message,
			regex:   re,
			enabled: p.Enabled,
		})
	}

	return dn
}

// UpdateConfig replaces the notifier's configuration and recompiles patterns.
// The dedup map (lastSent) is preserved so cooldowns survive config reloads.
func (dn *DesktopNotifier) UpdateConfig(cfg config.DesktopNotificationsConfig) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	dn.cfg = cfg

	dn.patterns = nil
	for _, p := range cfg.Patterns {
		escaped := regexp.QuoteMeta(p.Pattern)
		escaped = strings.ReplaceAll(escaped, `\*`, `.*`)
		escaped = strings.ReplaceAll(escaped, `\?`, `.`)
		re, err := regexp.Compile("(?i)" + escaped)
		if err != nil {
			continue
		}
		dn.patterns = append(dn.patterns, &compiledNotifyPattern{
			pattern: p.Pattern,
			title:   p.Title,
			message: p.Message,
			regex:   re,
			enabled: p.Enabled,
		})
	}
}

// CheckHealth sends a desktop notification if health drops below threshold.
func (dn *DesktopNotifier) CheckHealth(health int) {
	if !dn.cfg.HealthBelow.Enabled {
		return
	}
	if health <= dn.cfg.HealthBelow.Threshold {
		dn.send("Health Warning", fmt.Sprintf("Health at %d%%", health), "health")
	}
}

// CheckFatigue sends a desktop notification if fatigue drops below threshold.
func (dn *DesktopNotifier) CheckFatigue(fatigue int) {
	if !dn.cfg.FatigueBelow.Enabled {
		return
	}
	if fatigue <= dn.cfg.FatigueBelow.Threshold {
		dn.send("Fatigue Warning", fmt.Sprintf("Fatigue at %d%%", fatigue), "fatigue")
	}
}

// CheckText sends a desktop notification if text matches any enabled pattern.
func (dn *DesktopNotifier) CheckText(text string) {
	for _, p := range dn.patterns {
		if !p.enabled {
			continue
		}
		if p.regex.MatchString(text) {
			title := p.title
			if title == "" {
				title = "Alert"
			}
			msg := p.message
			if msg == "" {
				msg = text
			}
			dn.send(title, msg, "pattern:"+p.pattern)
			return // one notification per text line
		}
	}
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
}

// send dispatches a desktop notification with rate limiting.
func (dn *DesktopNotifier) send(title, message, dedupKey string) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	now := time.Now()
	if last, ok := dn.lastSent[dedupKey]; ok {
		if now.Sub(last) < dn.cooldown {
			return
		}
	}
	dn.lastSent[dedupKey] = now

	go sendDesktopNotification(title, message)
}

// sendDesktopNotification sends a notification using the platform's native mechanism.
// All arguments are passed as separate command args to avoid shell injection.
func sendDesktopNotification(title, message string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		// notify-send takes title and message as separate args — no shell.
		cmd = exec.Command("notify-send", title, message)
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
		cmd = exec.Command("osascript", "-e", script)
	case "windows":
		// Use PowerShell with escaped single quotes.
		escapedTitle := strings.ReplaceAll(title, "'", "''")
		escapedMsg := strings.ReplaceAll(message, "'", "''")
		ps := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; ` +
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent(0); ` +
			`$text = $template.GetElementsByTagName('text'); ` +
			`$text.Item(0).AppendChild($template.CreateTextNode('` + escapedTitle + `')) | Out-Null; ` +
			`$text.Item(1).AppendChild($template.CreateTextNode('` + escapedMsg + `')) | Out-Null; ` +
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); ` +
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Praetor').Show($toast)`
		cmd = exec.Command("powershell", "-Command", ps)
	default:
		log.Printf("[NOTIFY] desktop notifications not supported on %s", runtime.GOOS)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[NOTIFY] desktop notification failed: %v", err)
	}
	// Reap the child process to avoid zombies (#4).
	go cmd.Wait()
}
