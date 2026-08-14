package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSessionLogger_SingleLine(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}

	ts := time.Date(2026, 4, 3, 14, 30, 15, 0, time.UTC)
	sl.Log(ts, "You see grassy hillside.")
	sl.Close()

	content := readSessionLog(t, dir)
	if !strings.Contains(content, "[14:30:15] You see grassy hillside.") {
		t.Errorf("single line not found in log:\n%s", content)
	}
}

func TestSessionLogger_MultiLine(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}

	ts := time.Date(2026, 4, 3, 14, 30, 15, 0, time.UTC)
	sl.Log(ts, "A heavily battle-scarred bare-chested bandit arrives.\nHe is wielding a bronze sword.")
	sl.Close()

	content := readSessionLog(t, dir)
	if !strings.Contains(content, "[14:30:15] A heavily battle-scarred bare-chested bandit arrives.\nHe is wielding a bronze sword.") {
		t.Errorf("multi-line text not preserved in log:\n%s", content)
	}
}

func TestSessionLogger_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}

	t1 := time.Date(2026, 4, 3, 14, 30, 15, 0, time.UTC)
	t2 := time.Date(2026, 4, 3, 14, 30, 18, 0, time.UTC)
	t3 := time.Date(2026, 4, 3, 14, 30, 22, 0, time.UTC)

	sl.Log(t1, "You attack the bandit.")
	sl.Log(t2, "[Success: 5, Roll: 42] You thrust your iron sword forward.")
	sl.Log(t3, "You are no longer busy.")
	sl.Close()

	content := readSessionLog(t, dir)
	lines := strings.Split(content, "\n")

	// Find the three log entries.
	var found int
	for _, line := range lines {
		if strings.Contains(line, "[14:30:15] You attack the bandit.") {
			found++
		}
		if strings.Contains(line, "[14:30:18] [Success: 5, Roll: 42]") {
			found++
		}
		if strings.Contains(line, "[14:30:22] You are no longer busy.") {
			found++
		}
	}
	if found != 3 {
		t.Errorf("expected 3 entries, found %d in:\n%s", found, content)
	}
}

func TestSessionLogger_HeaderFooter(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}

	sl.Log(time.Now(), "test")
	sl.Close()

	content := readSessionLog(t, dir)
	if !strings.Contains(content, "=== Session started") {
		t.Error("missing session header")
	}
	if !strings.Contains(content, "=== Session ended") {
		t.Error("missing session footer")
	}
}

func TestSessionLogger_Disabled(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(false, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}

	sl.Log(time.Now(), "should not appear")
	sl.Close()

	// No file should be created.
	entries, _ := os.ReadDir(dir)
	if len(entries) > 0 {
		t.Errorf("disabled logger should not create files, found %d", len(entries))
	}
}

func TestSessionLogger_Filename(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}
	sl.Close()

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.HasPrefix(name, "session_") || !strings.HasSuffix(name, ".log") {
		t.Errorf("unexpected filename: %s", name)
	}
}

func TestSessionLogger_ReconfigureEnableAndDisable(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(false, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}
	if err := sl.Reconfigure(true, dir); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if !sl.Enabled() {
		t.Fatal("logger should be enabled")
	}
	sl.Log(time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC), "enabled line")
	if err := sl.Reconfigure(false, dir); err != nil {
		t.Fatalf("disable: %v", err)
	}
	sl.Log(time.Now(), "must not be logged")

	content := readSessionLog(t, dir)
	if !strings.Contains(content, "[01:02:03] enabled line") {
		t.Fatalf("enabled text missing:\n%s", content)
	}
	if strings.Contains(content, "must not be logged") {
		t.Fatalf("disabled text was logged:\n%s", content)
	}
	if !strings.Contains(content, "=== Session ended") {
		t.Fatalf("disable did not close the transcript:\n%s", content)
	}
}

func TestSessionLogger_ReconfigureDirectoryWhileEnabled(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	sl, err := NewSessionLogger(true, dirA)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}
	sl.Log(time.Now(), "in A")
	if err := sl.Reconfigure(true, dirB); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	sl.Log(time.Now(), "in B")
	if err := sl.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	a := readSessionLog(t, dirA)
	b := readSessionLog(t, dirB)
	if !strings.Contains(a, "in A") || strings.Contains(a, "in B") {
		t.Fatalf("unexpected first transcript:\n%s", a)
	}
	if !strings.Contains(b, "in B") || strings.Contains(b, "in A") {
		t.Fatalf("unexpected second transcript:\n%s", b)
	}
}

func TestSessionLogger_ReconfigureFailureKeepsCurrentFile(t *testing.T) {
	dir := t.TempDir()
	sl, err := NewSessionLogger(true, dir)
	if err != nil {
		t.Fatalf("NewSessionLogger: %v", err)
	}
	badPath := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(badPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sl.Reconfigure(true, badPath); err == nil {
		t.Fatal("expected reconfigure failure")
	}
	if !sl.Enabled() || sl.Directory() != dir {
		t.Fatalf("existing logger changed after failed reconfigure: enabled=%v dir=%q", sl.Enabled(), sl.Directory())
	}
	sl.Log(time.Now(), "still active")
	_ = sl.Close()
	if content := readSessionLog(t, dir); !strings.Contains(content, "still active") {
		t.Fatalf("old logger stopped after failure:\n%s", content)
	}
}

// readSessionLog reads the first .log file in dir.
func readSessionLog(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("reading log: %v", err)
			}
			return string(data)
		}
	}
	t.Fatal("no .log file found")
	return ""
}
