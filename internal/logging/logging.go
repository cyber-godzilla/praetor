package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger wraps slog and manages the log file lifecycle with size-based rotation.
type Logger struct {
	*slog.Logger
	writer io.WriteCloser
}

// New creates a structured logger that writes to a file in the given directory
// with size-based rotation. By default, the current file is renamed to .1 (one
// backup) when it reaches maxSizeMB. When retain is true, every segment is kept
// as a collision-safe, timestamped archive instead.
// The level string maps to slog levels: debug, info, warn, error.
// Also redirects the standard log package to this logger for compatibility.
func New(dir, filename, level string, maxSizeMB int, retain bool) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	if maxSizeMB <= 0 {
		maxSizeMB = 5
	}

	maxBytes := int64(maxSizeMB) * 1024 * 1024
	var writer io.WriteCloser
	var err error
	if retain {
		writer, err = newArchiveWriter(dir, filename, maxBytes)
	} else {
		writer, err = newRotatingWriter(filepath.Join(dir, filename), maxBytes)
	}
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

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: slogLevel,
	})

	logger := slog.New(handler)

	// Redirect standard log package to write through slog.
	log.SetOutput(&slogWriter{logger: logger})
	log.SetFlags(0) // slog handles timestamps

	return &Logger{Logger: logger, writer: writer}, nil
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

	// Degraded mode (a prior rotate/reopen failed): try to recover before writing
	// so logging resumes once the underlying problem clears, rather than staying
	// dead until restart.
	if rw.file == nil && !rw.reopen() {
		return rw.dropToStderr(p)
	}

	// Check if rotation is needed.
	if rw.written+int64(len(p)) > rw.maxBytes {
		if err := rw.rotate(); err != nil {
			fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
		}
		// rotate() may have left us degraded (nil file). Don't write to a closed
		// handle — drop this line to stderr and stay recoverable.
		if rw.file == nil {
			return rw.dropToStderr(p)
		}
	}

	n, err := rw.file.Write(p)
	rw.written += int64(n)
	return n, err
}

// dropToStderr writes a log line to stderr when the file is unavailable
// (degraded mode) so the line isn't lost, reporting success so slog doesn't
// error. Caller holds rw.mu.
func (rw *rotatingWriter) dropToStderr(p []byte) (int, error) {
	fmt.Fprint(os.Stderr, string(p))
	return len(p), nil
}

// reopen (re)opens the log path, seeding the written counter from the file size.
// On failure it leaves rw.file nil (degraded mode) and returns false. Caller
// holds rw.mu.
func (rw *rotatingWriter) reopen() bool {
	f, err := os.OpenFile(rw.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		rw.file = nil
		return false
	}
	rw.file = f
	if info, statErr := f.Stat(); statErr == nil {
		rw.written = info.Size()
	} else {
		rw.written = 0
	}
	return true
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
	// Close the current file (ignore the error — we're replacing it) and clear
	// the handle so a failure below can never leave us writing to a closed fd.
	if rw.file != nil {
		rw.file.Close()
		rw.file = nil
	}

	// Rename current → .1 (overwriting any existing backup).
	backup := rw.path + ".1"
	os.Remove(backup)
	if err := os.Rename(rw.path, backup); err != nil {
		// Rename failed: reopen the original so logging continues, but still
		// report the failure.
		rw.reopen()
		return err
	}

	// Open a fresh file at the original path. On failure we stay in degraded mode
	// (file nil); Write recovers on a later call once the problem clears.
	if !rw.reopen() {
		return fmt.Errorf("reopening log after rotate: %s", rw.path)
	}
	rw.written = 0
	return nil
}

// archiveWriter starts a new timestamped file when the current file exceeds
// maxBytes. Earlier files are never renamed, overwritten, or pruned.
type archiveWriter struct {
	mu       sync.Mutex
	file     *os.File
	dir      string
	stem     string
	path     string
	maxBytes int64
	written  int64
	now      func() time.Time
}

func newArchiveWriter(dir, filename string, maxBytes int64) (*archiveWriter, error) {
	return newArchiveWriterWithClock(dir, filename, maxBytes, time.Now)
}

func newArchiveWriterWithClock(dir, filename string, maxBytes int64, now func() time.Time) (*archiveWriter, error) {
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	if stem == "" || stem == "." || filepath.Base(stem) != stem {
		return nil, fmt.Errorf("invalid archive log filename %q", filename)
	}
	aw := &archiveWriter{
		dir:      dir,
		stem:     stem,
		maxBytes: maxBytes,
		now:      now,
	}
	if err := aw.openNext(); err != nil {
		return nil, err
	}
	return aw, nil
}

func (aw *archiveWriter) Write(p []byte) (int, error) {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	// A previous open may have failed. Retry on later writes so logging resumes
	// if the state directory becomes writable again.
	if aw.file == nil {
		if err := aw.openNext(); err != nil {
			return aw.dropToStderr(p)
		}
	}

	// Do not create an empty segment when one unusually large slog record is
	// larger than the configured segment size.
	if aw.written > 0 && aw.written+int64(len(p)) > aw.maxBytes {
		if err := aw.rotate(); err != nil {
			fmt.Fprintf(os.Stderr, "log archive rotation failed: %v\n", err)
		}
		if aw.file == nil {
			return aw.dropToStderr(p)
		}
	}

	n, err := aw.file.Write(p)
	aw.written += int64(n)
	return n, err
}

func (aw *archiveWriter) dropToStderr(p []byte) (int, error) {
	fmt.Fprint(os.Stderr, string(p))
	return len(p), nil
}

// openNext uses O_EXCL so concurrent Praetor processes or multiple rotations
// within the same second cannot append to or overwrite one another's history.
func (aw *archiveWriter) openNext() error {
	timestamp := aw.now().Format("2006-01-02_15-04-05")
	for sequence := 0; ; sequence++ {
		name := fmt.Sprintf("%s_%s.log", aw.stem, timestamp)
		if sequence > 0 {
			name = fmt.Sprintf("%s_%s_%02d.log", aw.stem, timestamp, sequence)
		}
		path := filepath.Join(aw.dir, name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			aw.file = f
			aw.path = path
			aw.written = 0
			return nil
		}
		if os.IsExist(err) {
			continue
		}
		aw.file = nil
		return err
	}
}

func (aw *archiveWriter) Close() error {
	aw.mu.Lock()
	defer aw.mu.Unlock()
	if aw.file == nil {
		return nil
	}
	err := aw.file.Close()
	aw.file = nil
	return err
}

func (aw *archiveWriter) rotate() error {
	if aw.file != nil {
		if err := aw.file.Close(); err != nil {
			aw.file = nil
			return err
		}
		aw.file = nil
	}
	return aw.openNext()
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
	// Route the game transcript and typed input ([RECV:*]/[SEND:*]) to debug so
	// the default info-level app log doesn't silently duplicate the session
	// transcript — a privacy footgun, since typed input can include an accidental
	// password paste. Operational/lifecycle lines stay at info.
	if strings.HasPrefix(msg, "[RECV:") || strings.HasPrefix(msg, "[SEND:") {
		w.logger.Debug(msg)
	} else {
		w.logger.Info(msg)
	}
	return len(p), nil
}
