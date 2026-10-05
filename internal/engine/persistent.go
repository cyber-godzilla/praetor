package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cyber-godzilla/praetor/internal/atomicfile"
)

const persistentFileName = "persistent_state.json"

// PersistentStore manages JSON file I/O for persistent state with debounced writes.
type PersistentStore struct {
	mu            sync.Mutex
	dataDir       string
	username      string
	dirty         bool
	closed        bool
	debounceDelay time.Duration
	debounceTimer *time.Timer
	snapshotFunc  func() map[string]interface{}
	flushes       sync.WaitGroup
	closeDone     chan struct{}
}

// NewPersistentStore creates a new store for the given user.
func NewPersistentStore(dataDir, username string) *PersistentStore {
	return &PersistentStore{
		dataDir:       dataDir,
		username:      username,
		debounceDelay: 5 * time.Second,
		closeDone:     make(chan struct{}),
	}
}

// SetSnapshotFunc sets the function called to get the current persistent state snapshot.
func (ps *PersistentStore) SetSnapshotFunc(fn func() map[string]interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.snapshotFunc = fn
}

// Load reads the persistent state for the current user from disk.
// Returns an empty map if the file doesn't exist.
func (ps *PersistentStore) Load() (map[string]interface{}, error) {
	filePath := filepath.Join(ps.dataDir, persistentFileName)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("reading persistent state: %w", err)
	}

	var allUsers map[string]map[string]interface{}
	if err := json.Unmarshal(raw, &allUsers); err != nil {
		return nil, fmt.Errorf("parsing persistent state: %w", err)
	}

	userData, ok := allUsers[ps.username]
	if !ok {
		return make(map[string]interface{}), nil
	}
	return userData, nil
}

// Save writes the persistent state for the current user to disk,
// preserving other users' data.
func (ps *PersistentStore) Save(data map[string]interface{}) error {
	filePath := filepath.Join(ps.dataDir, persistentFileName)

	allUsers := make(map[string]map[string]interface{})
	raw, err := os.ReadFile(filePath)
	if err == nil {
		if uerr := json.Unmarshal(raw, &allUsers); uerr != nil {
			// The existing file is corrupt. Merging from empty would drop every
			// other account's persistent data on this write, so refuse — but
			// preserve the original bytes in a .corrupt sidecar for recovery.
			sidecar := filePath + ".corrupt"
			if _, statErr := os.Stat(sidecar); os.IsNotExist(statErr) {
				_ = os.WriteFile(sidecar, raw, 0644)
			}
			return fmt.Errorf("persistent state file corrupt (preserved at %s): %w", sidecar, uerr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading persistent state: %w", err)
	}

	allUsers[ps.username] = data

	out, err := json.MarshalIndent(allUsers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling persistent state: %w", err)
	}

	if err := os.MkdirAll(ps.dataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	// Atomic write so a crash can't truncate the shared multi-account file.
	return atomicfile.Write(filePath, out, 0644)
}

// MarkDirty signals that persistent state has changed and should be flushed.
func (ps *PersistentStore) MarkDirty() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return
	}
	ps.dirty = true

	if ps.debounceTimer != nil {
		ps.debounceTimer.Stop()
	}
	ps.debounceTimer = time.AfterFunc(ps.debounceDelay, func() {
		ps.Flush()
	})
}

// Flush writes the current persistent state to disk immediately.
func (ps *PersistentStore) Flush() {
	ps.mu.Lock()
	if ps.closed || !ps.dirty {
		ps.mu.Unlock()
		return
	}
	ps.dirty = false
	fn := ps.snapshotFunc
	ps.flushes.Add(1)
	ps.mu.Unlock()
	defer ps.flushes.Done()

	if fn == nil {
		return
	}

	data := fn()
	if err := ps.Save(data); err != nil {
		log.Printf("[PERSIST] flush error: %v", err)
	}
}

// Close prevents future dirty marks, stops the debounce timer, waits for any
// flush already in flight, and synchronously saves the latest dirty snapshot.
// It is safe to call more than once. Callers must not hold the engine mutex:
// the snapshot function acquires it while copying Lua-backed state.
func (ps *PersistentStore) Close() {
	ps.mu.Lock()
	if ps.closed {
		done := ps.closeDone
		ps.mu.Unlock()
		<-done
		return
	}
	ps.closed = true
	if ps.debounceTimer != nil {
		ps.debounceTimer.Stop()
		ps.debounceTimer = nil
	}
	dirty := ps.dirty
	ps.dirty = false
	fn := ps.snapshotFunc
	ps.mu.Unlock()

	defer close(ps.closeDone)
	ps.flushes.Wait()
	if !dirty || fn == nil {
		return
	}
	data := fn()
	if err := ps.Save(data); err != nil {
		log.Printf("[PERSIST] close flush error: %v", err)
	}
}
