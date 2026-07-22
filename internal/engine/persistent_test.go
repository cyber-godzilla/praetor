package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentStore_Save_CorruptFileDoesNotClobber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, persistentFileName)

	// Two users on disk, then the file gets corrupted.
	if err := os.WriteFile(path, []byte(`{"alice":{"k":1},"bob":{"k":2}}`), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{corrupt`), 0644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	ps := NewPersistentStore(dir, "alice")
	if err := ps.Save(map[string]interface{}{"k": 99}); err == nil {
		t.Fatal("Save over a corrupt file succeeded; every other account's data would be dropped")
	}

	// Evidence preserved in a sidecar, and the live file untouched (not clobbered).
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Errorf("no .corrupt sidecar preserving the original bytes: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live file: %v", err)
	}
	if string(raw) != `{corrupt` {
		t.Errorf("live file was overwritten: %q", raw)
	}
}

func TestPersistentStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewPersistentStore(dir, "TestUser")

	data := map[string]interface{}{
		"armor_absorb":   map[string]interface{}{"leather cuirass": float64(47)},
		"lifetime_kills": float64(100),
	}
	if err := store.Save(data); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 keys, got %d", len(loaded))
	}
	armor, ok := loaded["armor_absorb"].(map[string]interface{})
	if !ok {
		t.Fatal("armor_absorb not a map")
	}
	if armor["leather cuirass"] != float64(47) {
		t.Errorf("leather cuirass = %v, want 47", armor["leather cuirass"])
	}
}

func TestPersistentStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewPersistentStore(dir, "TestUser")

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if loaded == nil {
		t.Error("expected non-nil empty map")
	}
}

func TestPersistentStore_MultiUser(t *testing.T) {
	dir := t.TempDir()

	store1 := NewPersistentStore(dir, "User1")
	store1.Save(map[string]interface{}{"kills": float64(10)}) //nolint:errcheck

	store2 := NewPersistentStore(dir, "User2")
	store2.Save(map[string]interface{}{"kills": float64(20)}) //nolint:errcheck

	loaded1, _ := store1.Load()
	if loaded1["kills"] != float64(10) {
		t.Errorf("User1 kills = %v, want 10", loaded1["kills"])
	}

	loaded2, _ := store2.Load()
	if loaded2["kills"] != float64(20) {
		t.Errorf("User2 kills = %v, want 20", loaded2["kills"])
	}
}

func TestPersistentStore_Debounce(t *testing.T) {
	dir := t.TempDir()
	store := NewPersistentStore(dir, "TestUser")
	store.debounceDelay = 100 * time.Millisecond

	data := map[string]interface{}{"key": float64(1)}
	store.SetSnapshotFunc(func() map[string]interface{} { return data })
	store.MarkDirty()

	filePath := filepath.Join(dir, "persistent_state.json")
	if _, err := os.Stat(filePath); err == nil {
		t.Error("file should not exist before debounce fires")
	}

	time.Sleep(200 * time.Millisecond)

	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("file not written after debounce: %v", err)
	}
	var allData map[string]map[string]interface{}
	json.Unmarshal(raw, &allData) //nolint:errcheck
	if allData["TestUser"]["key"] != float64(1) {
		t.Errorf("debounced data = %v, want 1", allData["TestUser"]["key"])
	}
}
