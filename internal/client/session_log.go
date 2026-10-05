package client

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionLogger is a reconfigurable facade over an asynchronous transcript
// writer. Reconfiguration opens the replacement before swapping it in, so a
// bad new path cannot interrupt the active transcript.
type SessionLogger struct {
	mu      sync.Mutex
	worker  *sessionLogWorker
	enabled bool
	dir     string
}

type sessionLogWorker struct {
	mu       sync.Mutex
	file     *os.File
	writer   *bufio.Writer
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

// NewSessionLogger creates a session logger. If enabled, it creates a
// timestamped log file in dir.
func NewSessionLogger(enabled bool, dir string) (*SessionLogger, error) {
	sl := &SessionLogger{dir: dir}
	if err := sl.Reconfigure(enabled, dir); err != nil {
		return nil, err
	}
	return sl, nil
}

// Log queues a timestamped line without blocking the game reader on file I/O.
func (sl *SessionLogger) Log(timestamp time.Time, text string) {
	sl.mu.Lock()
	worker := sl.worker
	sl.mu.Unlock()
	if worker != nil {
		worker.log(timestamp, text)
	}
}

// Close drains and closes the active transcript.
func (sl *SessionLogger) Close() error {
	sl.mu.Lock()
	worker := sl.worker
	sl.worker = nil
	sl.enabled = false
	sl.mu.Unlock()
	if worker == nil {
		return nil
	}
	return worker.close()
}

// Reconfigure applies logging enabled/path changes immediately. Enabling opens
// a new transcript; changing directories closes the old transcript after the
// replacement is ready. A failed open leaves the existing logger untouched.
func (sl *SessionLogger) Reconfigure(enabled bool, dir string) error {
	if !enabled {
		sl.mu.Lock()
		old := sl.worker
		sl.worker = nil
		sl.enabled = false
		sl.dir = dir
		sl.mu.Unlock()
		if old != nil {
			return old.close()
		}
		return nil
	}
	if dir == "" {
		return fmt.Errorf("session log directory is empty")
	}

	sl.mu.Lock()
	if sl.enabled && sl.worker != nil && filepath.Clean(sl.dir) == filepath.Clean(dir) {
		sl.mu.Unlock()
		return nil
	}
	sl.mu.Unlock()

	replacement, err := openSessionLog(dir)
	if err != nil {
		return err
	}

	sl.mu.Lock()
	old := sl.worker
	sl.worker = replacement
	sl.enabled = true
	sl.dir = dir
	sl.mu.Unlock()
	if old != nil {
		return old.close()
	}
	return nil
}

// Enabled reports whether transcript logging is currently active.
func (sl *SessionLogger) Enabled() bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.enabled && sl.worker != nil
}

// Directory returns the configured transcript directory.
func (sl *SessionLogger) Directory() string {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	return sl.dir
}

func openSessionLog(dir string) (*sessionLogWorker, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating session log dir: %w", err)
	}
	ts := time.Now().Format("2006-01-02_15-04-05")
	path := filepath.Join(dir, fmt.Sprintf("session_%s.log", ts))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening session log: %w", err)
	}
	writer := bufio.NewWriterSize(file, 64*1024)
	if _, err := fmt.Fprintf(writer, "=== Session started %s ===\n\n", time.Now().Format("2006-01-02 15:04:05")); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("writing session log header: %w", err)
	}
	worker := &sessionLogWorker{
		file: file, writer: writer, wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go worker.run()
	return worker, nil
}

func (w *sessionLogWorker) log(timestamp time.Time, text string) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	if len(w.queue) >= maxSessionLogQueue {
		w.dropped++
		w.mu.Unlock()
		return
	}
	w.queue = append(w.queue, sessionLogEntry{timestamp: timestamp, text: text})
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *sessionLogWorker) run() {
	defer close(w.done)
	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()
	for {
		select {
		case <-w.wake:
		case <-flushTicker.C:
			_ = w.writer.Flush()
		}
		for {
			w.mu.Lock()
			if len(w.queue) == 0 {
				closed := w.closed
				w.mu.Unlock()
				if closed {
					w.finish()
					return
				}
				break
			}
			entries := w.queue
			dropped := w.dropped
			w.queue = nil
			w.dropped = 0
			w.mu.Unlock()
			if dropped > 0 {
				_, _ = fmt.Fprintf(w.writer, "=== %d session log line(s) dropped under I/O pressure ===\n", dropped)
			}
			for _, entry := range entries {
				_, _ = fmt.Fprintf(w.writer, "[%s] %s\n", entry.timestamp.Format("15:04:05"), entry.text)
			}
		}
	}
}

func (w *sessionLogWorker) finish() {
	_, _ = fmt.Fprintf(w.writer, "\n=== Session ended %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	err := w.writer.Flush()
	if closeErr := w.file.Close(); err == nil {
		err = closeErr
	}
	w.mu.Lock()
	w.closeErr = err
	w.mu.Unlock()
}

func (w *sessionLogWorker) close() error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		select {
		case w.wake <- struct{}{}:
		default:
		}
	}
	done := w.done
	w.mu.Unlock()
	<-done
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeErr
}
