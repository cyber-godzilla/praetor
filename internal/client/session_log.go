package client

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionLogger writes timestamped game text to a log file for play session records.
type SessionLogger struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
	enabled  bool
	closed   bool
	queue    []sessionLogEntry
	dropped  int
	wake     chan struct{}
	done     chan struct{}
	closeErr error
}

const maxSessionLogQueue = 8192

type sessionLogEntry struct {
	timestamp time.Time
	text      string
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

	writer := bufio.NewWriterSize(f, 64*1024)
	fmt.Fprintf(writer, "=== Session started %s ===\n\n", time.Now().Format("2006-01-02 15:04:05"))
	sl := &SessionLogger{
		file:    f,
		writer:  writer,
		enabled: true,
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	go sl.run()
	return sl, nil
}

// Log writes a timestamped line of game text to the session log.
func (sl *SessionLogger) Log(timestamp time.Time, text string) {
	if !sl.enabled {
		return
	}
	sl.mu.Lock()
	if sl.closed {
		sl.mu.Unlock()
		return
	}
	if len(sl.queue) >= maxSessionLogQueue {
		sl.dropped++
		sl.mu.Unlock()
		return
	}
	sl.queue = append(sl.queue, sessionLogEntry{timestamp: timestamp, text: text})
	sl.mu.Unlock()
	select {
	case sl.wake <- struct{}{}:
	default:
	}
}

func (sl *SessionLogger) run() {
	defer close(sl.done)
	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()
	for {
		select {
		case <-sl.wake:
		case <-flushTicker.C:
			_ = sl.writer.Flush()
		}
		for {
			sl.mu.Lock()
			if len(sl.queue) == 0 {
				closed := sl.closed
				sl.mu.Unlock()
				if closed {
					sl.finish()
					return
				}
				break
			}
			entries := sl.queue
			dropped := sl.dropped
			sl.queue = nil
			sl.dropped = 0
			sl.mu.Unlock()
			if dropped > 0 {
				fmt.Fprintf(sl.writer, "=== %d session log line(s) dropped under I/O pressure ===\n", dropped)
			}
			for _, entry := range entries {
				fmt.Fprintf(sl.writer, "[%s] %s\n", entry.timestamp.Format("15:04:05"), entry.text)
			}
		}
	}
}

func (sl *SessionLogger) finish() {
	fmt.Fprintf(sl.writer, "\n=== Session ended %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	err := sl.writer.Flush()
	if closeErr := sl.file.Close(); err == nil {
		err = closeErr
	}
	sl.mu.Lock()
	sl.closeErr = err
	sl.mu.Unlock()
}

// Close flushes and closes the session log file.
func (sl *SessionLogger) Close() error {
	if !sl.enabled {
		return nil
	}
	sl.mu.Lock()
	if !sl.closed {
		sl.closed = true
		select {
		case sl.wake <- struct{}{}:
		default:
		}
	}
	done := sl.done
	sl.mu.Unlock()
	<-done
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.closeErr
}
