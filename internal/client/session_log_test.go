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
