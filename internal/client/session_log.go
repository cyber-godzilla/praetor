package client

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionLogger writes timestamped game text to a log file for play session records.
type SessionLogger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool
	dir     string
}

// NewSessionLogger creates a session logger. If enabled, it creates a timestamped
// log file in the given directory. The filename format is session_YYYY-MM-DD_HH-MM-SS.log.
func NewSessionLogger(enabled bool, dir string) (*SessionLogger, error) {
	sl := &SessionLogger{dir: dir}
	if err := sl.Reconfigure(enabled, dir); err != nil {
		return nil, err
	}
	return sl, nil
}

// Log writes a timestamped line of game text to the session log.
func (sl *SessionLogger) Log(timestamp time.Time, text string) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if !sl.enabled || sl.file == nil {
		return
	}

	ts := timestamp.Format("15:04:05")
	_, _ = fmt.Fprintf(sl.file, "[%s] %s\n", ts, text)
}

// Close flushes and closes the session log file.
func (sl *SessionLogger) Close() error {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.closeLocked()
}

// Reconfigure applies logging enabled/path changes immediately. Enabling opens
// a new timestamped transcript; changing the directory while enabled closes
// the current transcript with a footer and starts a new one in the requested
// directory. If the new file cannot be opened, the existing logger is left
// untouched.
func (sl *SessionLogger) Reconfigure(enabled bool, dir string) error {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	if !enabled {
		err := sl.closeLocked()
		sl.dir = dir
		return err
	}
	if dir == "" {
		return fmt.Errorf("session log directory is empty")
	}
	if sl.enabled && sl.file != nil && filepath.Clean(sl.dir) == filepath.Clean(dir) {
		return nil
	}

	newFile, err := openSessionLog(dir)
	if err != nil {
		return err
	}

	oldFile := sl.file
	sl.file = newFile
	sl.enabled = true
	sl.dir = dir
	if oldFile != nil {
		_, _ = fmt.Fprintf(oldFile, "\n=== Session ended %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
		return oldFile.Close()
	}
	return nil
}

// Enabled reports whether transcript logging is currently active.
func (sl *SessionLogger) Enabled() bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled && sl.file != nil
}

// Directory returns the currently configured transcript directory.
func (sl *SessionLogger) Directory() string {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.dir
}

func openSessionLog(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating session log dir: %w", err)
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, fmt.Sprintf("session_%s.log", ts))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening session log: %w", err)
	}
	if _, err := fmt.Fprintf(f, "=== Session started %s ===\n\n", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("writing session log header: %w", err)
	}
	return f, nil
}

func (sl *SessionLogger) closeLocked() error {
	if sl.file == nil {
		sl.enabled = false
		return nil
	}

	f := sl.file
	sl.file = nil
	sl.enabled = false
	_, writeErr := fmt.Fprintf(f, "\n=== Session ended %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
