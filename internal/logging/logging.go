package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Logger wraps slog and manages the log file lifecycle with size-based rotation.
type Logger struct {
	*slog.Logger
	writer *rotatingWriter
}

// New creates a structured logger that writes to a file in the given directory
// with size-based rotation. When the log exceeds maxSizeMB, the current file
// is renamed to .1 (one backup) and a fresh file is started.
// The level string maps to slog levels: debug, info, warn, error.
// Also redirects the standard log package to this logger for compatibility.
func New(dir, filename, level string, maxSizeMB int) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	if maxSizeMB <= 0 {
		maxSizeMB = 5
	}

	path := filepath.Join(dir, filename)
	rw, err := newRotatingWriter(path, int64(maxSizeMB)*1024*1024)
	if err != nil {
		return nil, err
	}

	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(rw, &slog.HandlerOptions{
		Level: slogLevel,
	})

	logger := slog.New(handler)

	// Redirect standard log package to write through slog.
	log.SetOutput(&slogWriter{logger: logger})
	log.SetFlags(0) // slog handles timestamps

	return &Logger{Logger: logger, writer: rw}, nil
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	if l.writer != nil {
		return l.writer.Close()
	}
	return nil
}

// Writer returns the underlying writer for use as an io.Writer.
func (l *Logger) Writer() io.Writer {
	return l.writer
}

// rotatingWriter is an io.WriteCloser that rotates the log file when it
// exceeds maxBytes. Keeps one backup (.1 suffix).
type rotatingWriter struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxBytes int64
	written  int64
}

func newRotatingWriter(path string, maxBytes int64) (*rotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// Get current file size for accurate tracking.
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &rotatingWriter{
		file:     f,
		path:     path,
		maxBytes: maxBytes,
		written:  info.Size(),
	}, nil
}

func (rw *rotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Check if rotation is needed.
	if rw.written+int64(len(p)) > rw.maxBytes {
		if err := rw.rotate(); err != nil {
			// If rotation fails, keep writing to the current file.
			fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
		}
	}

	n, err := rw.file.Write(p)
	rw.written += int64(n)
	return n, err
}

func (rw *rotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	if rw.file != nil {
		return rw.file.Close()
	}
	return nil
}

func (rw *rotatingWriter) rotate() error {
	// Close current file.
	if err := rw.file.Close(); err != nil {
		return err
	}

	// Rename current → .1 (overwriting any existing backup).
	backup := rw.path + ".1"
	os.Remove(backup)
	if err := os.Rename(rw.path, backup); err != nil {
		// If rename fails, try to reopen the original.
		f, openErr := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			return openErr
		}
		rw.file = f
		return err
	}

	// Open fresh file.
	f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	rw.file = f
	rw.written = 0
	return nil
}

// slogWriter adapts slog.Logger to io.Writer for the standard log package.
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (int, error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	w.logger.Info(msg)
	return len(p), nil
}
