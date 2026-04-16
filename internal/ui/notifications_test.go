package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyber-godzilla/praetor/internal/config"
)

func notifyCfg() config.DesktopNotificationsConfig {
	return config.DesktopNotificationsConfig{
		HealthBelow:  config.ThresholdConfig{Enabled: true, Threshold: 25},
		FatigueBelow: config.ThresholdConfig{Enabled: false, Threshold: 10},
		Patterns: []config.NotifyPatternConfig{
			{Pattern: "dragon", Title: "Dragon!", Message: "A dragon appeared", Enabled: true},
			{Pattern: "treasure", Enabled: false},
		},
	}
}

func keyMsg(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func runeMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestToggleThreshold(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	// Cursor starts at 0 (health threshold), which is enabled.
	if !s.healthBelow.Enabled {
		t.Fatal("expected healthBelow to start enabled")
	}

	// Toggle off.
	s, _ = s.Update(keyMsg(tea.KeySpace))
	if s.healthBelow.Enabled {
		t.Fatal("expected healthBelow to be disabled after first toggle")
	}

	// Toggle back on.
	s, _ = s.Update(keyMsg(tea.KeySpace))
	if !s.healthBelow.Enabled {
		t.Fatal("expected healthBelow to be re-enabled after second toggle")
	}
}

func TestTogglePattern(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())

	// Navigate past 2 thresholds to reach first pattern (index 2).
	s, _ = s.Update(keyMsg(tea.KeyDown))
	s, _ = s.Update(keyMsg(tea.KeyDown))

	if !s.patterns[0].Enabled {
		t.Fatal("expected first pattern to start enabled")
	}

	// Toggle off.
	s, _ = s.Update(keyMsg(tea.KeySpace))
	if s.patterns[0].Enabled {
		t.Fatal("expected first pattern to be disabled after toggle")
	}

	// Toggle back on.
	s, _ = s.Update(keyMsg(tea.KeySpace))
	if !s.patterns[0].Enabled {
		t.Fatal("expected first pattern to be re-enabled after toggle")
	}
}

func TestEditThresholdValue(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	// Cursor at 0 (health threshold=25). Enter to edit.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if !s.editing {
		t.Fatal("expected editing to be true after Enter on threshold")
	}
	if s.editBuf != "25" {
		t.Fatalf("expected editBuf to be '25', got %q", s.editBuf)
	}

	// Backspace twice to clear "25".
	s, _ = s.Update(keyMsg(tea.KeyBackspace))
	s, _ = s.Update(keyMsg(tea.KeyBackspace))
	if s.editBuf != "" {
		t.Fatalf("expected editBuf to be empty after backspaces, got %q", s.editBuf)
	}

	// Type "50".
	s, _ = s.Update(runeMsg('5'))
	s, _ = s.Update(runeMsg('0'))
	if s.editBuf != "50" {
		t.Fatalf("expected editBuf to be '50', got %q", s.editBuf)
	}

	// Confirm.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editing {
		t.Fatal("expected editing to be false after confirming")
	}
	if s.healthBelow.Threshold != 50 {
		t.Fatalf("expected threshold to be 50, got %d", s.healthBelow.Threshold)
	}
}

func TestEditThresholdClamp(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	// Enter edit on health threshold.
	s, _ = s.Update(keyMsg(tea.KeyEnter))

	// Clear current value.
	s, _ = s.Update(keyMsg(tea.KeyBackspace))
	s, _ = s.Update(keyMsg(tea.KeyBackspace))

	// Type "200" (exceeds 100).
	s, _ = s.Update(runeMsg('2'))
	s, _ = s.Update(runeMsg('0'))
	s, _ = s.Update(runeMsg('0'))

	// Confirm.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.healthBelow.Threshold != 100 {
		t.Fatalf("expected threshold clamped to 100, got %d", s.healthBelow.Threshold)
	}
}

func TestAddPattern(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	initialCount := len(s.patterns)

	// Navigate to "Add new pattern..." which is at index: 2 thresholds + 2 patterns = index 4.
	for i := 0; i < 4; i++ {
		s, _ = s.Update(keyMsg(tea.KeyDown))
	}
	if s.items[s.cursor].kind != notifyItemAdd {
		t.Fatal("expected cursor to be on Add item")
	}

	// Enter to start adding.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if !s.editing {
		t.Fatal("expected editing to be true after Enter on Add")
	}
	if s.editField != fieldPattern {
		t.Fatalf("expected editField to be fieldPattern, got %d", s.editField)
	}

	// Type pattern name "goblin".
	for _, r := range "goblin" {
		s, _ = s.Update(runeMsg(r))
	}

	// Enter to advance to title.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editField != fieldTitle {
		t.Fatalf("expected editField to be fieldTitle, got %d", s.editField)
	}

	// Type title "Goblin Alert".
	for _, r := range "Goblin Alert" {
		s, _ = s.Update(runeMsg(r))
	}

	// Enter to advance to message.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editField != fieldMessage {
		t.Fatalf("expected editField to be fieldMessage, got %d", s.editField)
	}

	// Type message "A goblin appeared".
	for _, r := range "A goblin appeared" {
		s, _ = s.Update(runeMsg(r))
	}

	// Enter to finish editing.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editing {
		t.Fatal("expected editing to be false after finishing add")
	}

	if len(s.patterns) != initialCount+1 {
		t.Fatalf("expected %d patterns, got %d", initialCount+1, len(s.patterns))
	}

	newPattern := s.patterns[len(s.patterns)-1]
	if newPattern.Pattern != "goblin" {
		t.Fatalf("expected pattern 'goblin', got %q", newPattern.Pattern)
	}
	if newPattern.Title != "Goblin Alert" {
		t.Fatalf("expected title 'Goblin Alert', got %q", newPattern.Title)
	}
	if newPattern.Message != "A goblin appeared" {
		t.Fatalf("expected message 'A goblin appeared', got %q", newPattern.Message)
	}
	if !newPattern.Enabled {
		t.Fatal("expected new pattern to be Enabled")
	}
}

func TestAddEmptyPatternRemoves(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	initialCount := len(s.patterns)

	// Navigate to "Add new pattern...".
	for i := 0; i < 4; i++ {
		s, _ = s.Update(keyMsg(tea.KeyDown))
	}

	// Enter to start adding.
	s, _ = s.Update(keyMsg(tea.KeyEnter))

	// Enter immediately with empty pattern.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editing {
		t.Fatal("expected editing to be false after empty pattern confirm")
	}
	if len(s.patterns) != initialCount {
		t.Fatalf("expected %d patterns (empty pattern removed), got %d", initialCount, len(s.patterns))
	}
}

func TestDeletePatternWithConfirmation(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())

	// Navigate to first pattern (index 2).
	s, _ = s.Update(keyMsg(tea.KeyDown))
	s, _ = s.Update(keyMsg(tea.KeyDown))

	if s.items[s.cursor].kind != notifyItemPattern {
		t.Fatal("expected cursor to be on a pattern item")
	}

	// Press 'd' to start delete.
	s, _ = s.Update(runeMsg('d'))
	if !s.confirm {
		t.Fatal("expected confirm to be true after pressing d on a pattern")
	}

	// Press 'n' to cancel.
	s, _ = s.Update(runeMsg('n'))
	if s.confirm {
		t.Fatal("expected confirm to be false after pressing n")
	}
	if len(s.patterns) != 2 {
		t.Fatalf("expected 2 patterns after cancel, got %d", len(s.patterns))
	}

	// Press 'd' again, then 'y' to confirm.
	s, _ = s.Update(runeMsg('d'))
	if !s.confirm {
		t.Fatal("expected confirm to be true after pressing d again")
	}
	s, _ = s.Update(runeMsg('y'))
	if s.confirm {
		t.Fatal("expected confirm to be false after pressing y")
	}
	if len(s.patterns) != 1 {
		t.Fatalf("expected 1 pattern after delete, got %d", len(s.patterns))
	}
	// The remaining pattern should be "treasure".
	if s.patterns[0].Pattern != "treasure" {
		t.Fatalf("expected remaining pattern to be 'treasure', got %q", s.patterns[0].Pattern)
	}
}

func TestDeleteOnThresholdDoesNothing(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	// Cursor starts on threshold (index 0).
	s, _ = s.Update(runeMsg('d'))
	if s.confirm {
		t.Fatal("expected confirm to remain false when pressing d on a threshold")
	}
}

func TestEscReturnsConfig(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())

	// Toggle health off.
	s, _ = s.Update(keyMsg(tea.KeySpace))
	if s.healthBelow.Enabled {
		t.Fatal("expected healthBelow to be disabled")
	}

	// Esc to close.
	s, cmd := s.Update(keyMsg(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("expected a Cmd from Esc")
	}

	msg := cmd()
	closeMsg, ok := msg.(NotificationSettingsCloseMsg)
	if !ok {
		t.Fatalf("expected NotificationSettingsCloseMsg, got %T", msg)
	}
	if closeMsg.Config.HealthBelow.Enabled {
		t.Fatal("expected config HealthBelow to be disabled")
	}
	if closeMsg.Config.FatigueBelow.Threshold != 10 {
		t.Fatalf("expected fatigue threshold 10, got %d", closeMsg.Config.FatigueBelow.Threshold)
	}
	if len(closeMsg.Config.Patterns) != 2 {
		t.Fatalf("expected 2 patterns in returned config, got %d", len(closeMsg.Config.Patterns))
	}
	_ = s
}

func TestEscDuringEditCancels(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())

	// Enter edit on health threshold.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if !s.editing {
		t.Fatal("expected editing to be true")
	}

	// Type some digits.
	s, _ = s.Update(runeMsg('9'))
	s, _ = s.Update(runeMsg('9'))

	// Esc to cancel edit.
	s, cmd := s.Update(keyMsg(tea.KeyEscape))
	if s.editing {
		t.Fatal("expected editing to be false after Esc")
	}
	// Threshold should remain unchanged.
	if s.healthBelow.Threshold != 25 {
		t.Fatalf("expected threshold to remain 25, got %d", s.healthBelow.Threshold)
	}
	// Esc during edit should not produce a close command.
	if cmd != nil {
		t.Fatal("expected no Cmd from Esc during edit")
	}
}

func TestPatternFieldCycling(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())

	// Navigate to first pattern (index 2).
	s, _ = s.Update(keyMsg(tea.KeyDown))
	s, _ = s.Update(keyMsg(tea.KeyDown))

	// Enter to edit existing pattern.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if !s.editing {
		t.Fatal("expected editing to be true")
	}
	if s.editField != fieldPattern {
		t.Fatalf("expected editField=fieldPattern, got %d", s.editField)
	}
	if s.editBuf != "dragon" {
		t.Fatalf("expected editBuf='dragon', got %q", s.editBuf)
	}

	// Enter advances to fieldTitle.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editField != fieldTitle {
		t.Fatalf("expected editField=fieldTitle, got %d", s.editField)
	}
	if s.editBuf != "Dragon!" {
		t.Fatalf("expected editBuf='Dragon!', got %q", s.editBuf)
	}

	// Enter advances to fieldMessage.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editField != fieldMessage {
		t.Fatalf("expected editField=fieldMessage, got %d", s.editField)
	}
	if s.editBuf != "A dragon appeared" {
		t.Fatalf("expected editBuf='A dragon appeared', got %q", s.editBuf)
	}

	// Enter finishes editing.
	s, _ = s.Update(keyMsg(tea.KeyEnter))
	if s.editing {
		t.Fatal("expected editing to be false after completing all fields")
	}
}

func TestViewRenders(t *testing.T) {
	s := NewNotificationSettingsScreen(notifyCfg())
	s.SetSize(80, 24)
	output := s.View()

	for _, want := range []string{
		"Notification Settings",
		"Thresholds",
		"Patterns",
		"Add new pattern",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected View output to contain %q", want)
		}
	}
}
