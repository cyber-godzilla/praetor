package session

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func randomCredentialKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptedFileCredentialStoreRoundTripAndRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "credentials.enc")
	key := randomCredentialKey(t)
	store, err := NewEncryptedFileCredentialStore(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("bob", "bob-password"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("alice", "alice-password"); err != nil {
		t.Fatal(err)
	}

	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(accounts, ",") != "alice,bob" {
		t.Fatalf("accounts = %v", accounts)
	}

	reopened, err := NewEncryptedFileCredentialStore(path, key)
	if err != nil {
		t.Fatal(err)
	}
	password, err := reopened.GetAccount("alice")
	if err != nil || password != "alice-password" {
		t.Fatalf("reopened password=%q err=%v", password, err)
	}
	if err := reopened.RemoveAccount("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetAccount("alice"); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("removed account error = %v", err)
	}
	if err := reopened.RemoveAccount("bob"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty store was not removed: %v", err)
	}
}

func TestEncryptedFileCredentialStoreProtectsContentsAndPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "credentials.enc")
	store, err := NewEncryptedFileCredentialStore(path, randomCredentialKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("visible-username", "visible-password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("visible-username")) || bytes.Contains(data, []byte("visible-password")) {
		t.Fatalf("encrypted file contains plaintext account data: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential file mode = %o, want 600", info.Mode().Perm())
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("credential directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
}

func TestEncryptedFileCredentialStoreRejectsWrongKeyAndTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.enc")
	store, err := NewEncryptedFileCredentialStore(path, randomCredentialKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("alice", "password"); err != nil {
		t.Fatal(err)
	}

	wrongKeyStore, err := NewEncryptedFileCredentialStore(path, randomCredentialKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wrongKeyStore.ListAccounts(); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("wrong-key error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-8] ^= 1
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListAccounts(); err == nil {
		t.Fatal("tampered credential file was accepted")
	}
}

func TestEncryptedFileCredentialStoreUsesFreshNonce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.enc")
	store, err := NewEncryptedFileCredentialStore(path, randomCredentialKey(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("alice", "password"); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetAccount("alice", "password"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("rewriting identical data reused the encrypted representation")
	}
}

func TestEncryptedFileCredentialStoreSerializesConcurrentUpdates(t *testing.T) {
	store, err := NewEncryptedFileCredentialStore(
		filepath.Join(t.TempDir(), "credentials.enc"),
		randomCredentialKey(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	const count = 20
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i))
			if err := store.SetAccount(name, "password-"+name); err != nil {
				t.Errorf("SetAccount(%q): %v", name, err)
			}
		}(i)
	}
	wg.Wait()
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != count {
		t.Fatalf("account count = %d, want %d", len(accounts), count)
	}
}

func TestCredentialStoreFactoryEncryptedFile(t *testing.T) {
	key := randomCredentialKey(t)
	t.Setenv("PRAETOR_TEST_CREDENTIAL_KEY", base64.StdEncoding.EncodeToString(key))
	store, err := NewCredentialStore(CredentialStoreOptions{
		Backend:  CredentialBackendEncryptedFile,
		StateDir: t.TempDir(),
		KeyEnv:   "PRAETOR_TEST_CREDENTIAL_KEY",
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.Descriptor().Backend != CredentialBackendEncryptedFile {
		t.Fatalf("descriptor = %+v", store.Descriptor())
	}
	if _, exists := os.LookupEnv("PRAETOR_TEST_CREDENTIAL_KEY"); exists {
		t.Fatal("credential key remained in the process environment")
	}
}

func TestCredentialStoreFactoryRejectsMissingInvalidAndWrongKey(t *testing.T) {
	stateDir := t.TempDir()
	if _, err := NewCredentialStore(CredentialStoreOptions{
		Backend: CredentialBackendEncryptedFile, StateDir: stateDir, KeyEnv: "PRAETOR_MISSING_KEY",
	}); err == nil || !strings.Contains(err.Error(), "requires PRAETOR_MISSING_KEY") {
		t.Fatalf("missing-key error = %v", err)
	}

	t.Setenv("PRAETOR_BAD_KEY", "not-base64")
	if _, err := NewCredentialStore(CredentialStoreOptions{
		Backend: CredentialBackendEncryptedFile, StateDir: stateDir, KeyEnv: "PRAETOR_BAD_KEY",
	}); err == nil || !strings.Contains(err.Error(), "key must") {
		t.Fatalf("invalid-key error = %v", err)
	}

	path := filepath.Join(stateDir, "credentials.enc")
	firstKey := randomCredentialKey(t)
	first, err := NewEncryptedFileCredentialStore(path, firstKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.SetAccount("alice", "password"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRAETOR_WRONG_KEY", base64.StdEncoding.EncodeToString(randomCredentialKey(t)))
	if _, err := NewCredentialStore(CredentialStoreOptions{
		Backend: CredentialBackendEncryptedFile, FilePath: path, KeyEnv: "PRAETOR_WRONG_KEY",
	}); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("wrong-key startup error = %v", err)
	}
}

func TestDisabledCredentialStore(t *testing.T) {
	store, err := NewCredentialStore(CredentialStoreOptions{Backend: CredentialBackendDisabled})
	if err != nil {
		t.Fatal(err)
	}
	if store.Descriptor().CanStore {
		t.Fatal("disabled store claims it can persist credentials")
	}
	accounts, err := store.ListAccounts()
	if err != nil || len(accounts) != 0 {
		t.Fatalf("disabled accounts=%v err=%v", accounts, err)
	}
	if err := store.SetAccount("alice", "password"); !errors.Is(err, ErrCredentialStorageDisabled) {
		t.Fatalf("SetAccount error = %v", err)
	}
}
