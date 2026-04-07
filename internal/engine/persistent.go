package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const persistentFileName = "persistent_state.json"

// PersistentStore manages JSON file I/O for persistent state with debounced writes.
type PersistentStore struct {
	mu            sync.Mutex
	dataDir       string
	username      string
	dirty         bool
	debounceDelay time.Duration
	debounceTimer *time.Timer
	snapshotFunc  func() map[string]interface{}
}

// NewPersistentStore creates a new store for the given user.
func NewPersistentStore(dataDir, username string) *PersistentStore {
	return &PersistentStore{
		dataDir:       dataDir,
		username:      username,
		debounceDelay: 5 * time.Second,
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

	var allUsers map[string]map[string]interface{}
	raw, err := os.ReadFile(filePath)
	if err == nil {
		json.Unmarshal(raw, &allUsers) //nolint:errcheck
	}
	if allUsers == nil {
		allUsers = make(map[string]map[string]interface{})
	}

	allUsers[ps.username] = data

	out, err := json.MarshalIndent(allUsers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling persistent state: %w", err)
	}

	if err := os.MkdirAll(ps.dataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	if err := os.WriteFile(filePath, out, 0644); err != nil {
		return fmt.Errorf("writing persistent state: %w", err)
	}

	return nil
}

// MarkDirty signals that persistent state has changed and should be flushed.
func (ps *PersistentStore) MarkDirty() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
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
	if !ps.dirty {
		ps.mu.Unlock()
		return
	}
	ps.dirty = false
	fn := ps.snapshotFunc
	ps.mu.Unlock()

	if fn == nil {
		return
	}

	data := fn()
	if err := ps.Save(data); err != nil {
		log.Printf("[PERSIST] flush error: %v", err)
	}
}
