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
}

// NewSessionLogger creates a session logger. If enabled, it creates a timestamped
// log file in the given directory. The filename format is session_YYYY-MM-DD_HH-MM-SS.log.
func NewSessionLogger(enabled bool, dir string) (*SessionLogger, error) {
	if !enabled {
		return &SessionLogger{enabled: false}, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating session log dir: %w", err)
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, fmt.Sprintf("session_%s.log", ts))

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening session log: %w", err)
	}

	// Write header.
	fmt.Fprintf(f, "=== Session started %s ===\n\n", time.Now().Format("2006-01-02 15:04:05"))

	return &SessionLogger{file: f, enabled: true}, nil
}

// Log writes a timestamped line of game text to the session log.
func (sl *SessionLogger) Log(timestamp time.Time, text string) {
	if !sl.enabled || sl.file == nil {
		return
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	ts := timestamp.Format("15:04:05")
	fmt.Fprintf(sl.file, "[%s] %s\n", ts, text)
}

// Close flushes and closes the session log file.
func (sl *SessionLogger) Close() error {
	if !sl.enabled || sl.file == nil {
		return nil
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()

	fmt.Fprintf(sl.file, "\n=== Session ended %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	return sl.file.Close()
}
