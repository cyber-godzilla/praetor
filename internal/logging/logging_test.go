package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlogWriter_RoutesTranscriptToDebug(t *testing.T) {
	var buf bytes.Buffer
	infoLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	w := &slogWriter{logger: infoLogger}

	// The game transcript and typed input (which can include an accidental
	// password paste) must NOT be duplicated into the default info-level app log.
	w.Write([]byte("[RECV:TEXT] You see a sword.\n"))
	w.Write([]byte("[SEND:GAME] my-secret-password\n"))
	w.Write([]byte("[SEND:CMD] /mode combat\n"))
	if buf.Len() != 0 {
		t.Errorf("transcript lines were written at info level: %q", buf.String())
	}

	// Lifecycle/operational lines still log at info.
	w.Write([]byte("[CLIENT] connected\n"))
	if !strings.Contains(buf.String(), "connected") {
		t.Errorf("operational line was dropped at info level: %q", buf.String())
	}

	// At debug level, a developer can opt into the transcript.
	buf.Reset()
	dbg := &slogWriter{logger: slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))}
	dbg.Write([]byte("[RECV:TEXT] visible at debug\n"))
	if !strings.Contains(buf.String(), "visible at debug") {
		t.Error("transcript not written even at debug level")
	}
}

func TestRotatingWriter_RecoversFromReopenFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tec.log")
	rw, err := newRotatingWriter(path, 100) // tiny cap to force rotation
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer rw.Close()

	if _, err := rw.Write([]byte("under the cap\n")); err != nil {
		t.Fatalf("initial write: %v", err)
	}

	// Remove the directory so the next rotation's reopen fails (disk-full / dir
	// removed analogue). The open fd keeps working, but rename + reopen can't.
	os.RemoveAll(dir)

	// This write crosses the cap and triggers a rotate whose reopen fails. It
	// must not panic or write to a closed handle — degraded mode drops to stderr.
	big := make([]byte, 200)
	if _, err := rw.Write(big); err != nil {
		t.Fatalf("degraded write returned an error instead of no-op: %v", err)
	}
	// And another write while still degraded stays safe.
	if _, err := rw.Write([]byte("still degraded\n")); err != nil {
		t.Fatalf("second degraded write errored: %v", err)
	}

	// Restore the directory: a later write must recover and recreate the file.
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("restore dir: %v", err)
	}
	if _, err := rw.Write([]byte("recovered\n")); err != nil {
		t.Fatalf("did not recover after the directory was restored: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file was not recreated after recovery: %v", err)
	}
}

func TestRotatingWriter_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(path, 1024)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer rw.Close()

	rw.Write([]byte("hello world\n"))

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("expected 'hello world' in log, got: %s", data)
	}
}

func TestRotatingWriter_Rotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// Max 50 bytes — will rotate quickly.
	rw, err := newRotatingWriter(path, 50)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}
	defer rw.Close()

	// Write enough to trigger rotation.
	for i := 0; i < 5; i++ {
		rw.Write([]byte("this is a line of log text\n"))
	}

	// Backup file should exist.
	backup := path + ".1"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		t.Error("backup file (.1) not created after rotation")
	}

	// Both files should exist and have content.
	currentInfo, _ := os.Stat(path)
	backupInfo, _ := os.Stat(backup)
	if currentInfo.Size() == 0 {
		t.Error("current file should have content after rotation")
	}
	if backupInfo.Size() == 0 {
		t.Error("backup file should have content after rotation")
	}
}

func TestRotatingWriter_BackupOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	rw, err := newRotatingWriter(path, 30)
	if err != nil {
		t.Fatalf("newRotatingWriter: %v", err)
	}

	// Trigger multiple rotations.
	for i := 0; i < 10; i++ {
		rw.Write([]byte("rotation test line\n"))
	}
	rw.Close()

	// Should only have one backup (.1), not .2, .3, etc.
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "test.log") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 files (current + .1 backup), got %d", count)
	}
}

func TestLogger_New(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "app.log", "info", 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	l.Info("test message", "key", "value")

	data, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	content := string(data)
	if !strings.Contains(content, "test message") {
		t.Errorf("expected 'test message' in log, got: %s", content)
	}
	if !strings.Contains(content, "key=value") {
		t.Errorf("expected 'key=value' in log, got: %s", content)
	}
}

func TestLogger_DebugLevel(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "app.log", "debug", 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	l.Debug("debug msg")
	l.Info("info msg")

	data, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	content := string(data)
	if !strings.Contains(content, "debug msg") {
		t.Errorf("debug message should appear at debug level, got: %s", content)
	}
}

func TestLogger_InfoLevelFiltersDebug(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "app.log", "info", 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	l.Debug("debug msg")
	l.Info("info msg")

	data, _ := os.ReadFile(filepath.Join(dir, "app.log"))
	content := string(data)
	if strings.Contains(content, "debug msg") {
		t.Errorf("debug message should be filtered at info level, got: %s", content)
	}
	if !strings.Contains(content, "info msg") {
		t.Errorf("info message should appear, got: %s", content)
	}
}
