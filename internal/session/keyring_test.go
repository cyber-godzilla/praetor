package session

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyringStore_CorruptBlobDoesNotClobber(t *testing.T) {
	keyring.MockInit()

	// Seed a valid multi-account blob, then corrupt it (truncation/garbage).
	if err := keyring.Set(keyringService, keyringAccountKey, `{"alice":"a","bob":"b"}`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := keyring.Set(keyringService, keyringAccountKey, `{corrupt`); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	k := &KeyringStore{}

	// A corrupt blob must surface as an error, not silently read as "no accounts".
	if _, err := k.ListAccounts(); err == nil {
		t.Fatal("ListAccounts on a corrupt blob returned nil error")
	}

	// Writes must refuse rather than overwrite (which would destroy alice+bob).
	if err := k.SetAccount("carol", "c"); err == nil {
		t.Fatal("SetAccount on a corrupt blob succeeded; it clobbered stored accounts")
	}
	if err := k.RemoveAccount("alice"); err == nil {
		t.Fatal("RemoveAccount on a corrupt blob succeeded; it could delete the entry")
	}

	// The blob must be exactly as left — never overwritten or deleted.
	raw, err := keyring.Get(keyringService, keyringAccountKey)
	if err != nil {
		t.Fatalf("blob was deleted or unreadable: %v", err)
	}
	if raw != `{corrupt` {
		t.Fatalf("blob was overwritten: %q", raw)
	}
}

func TestKeyringStore_ValidBlobRoundTrips(t *testing.T) {
	keyring.MockInit()
	k := &KeyringStore{}

	if err := k.SetAccount("alice", "pa"); err != nil {
		t.Fatalf("SetAccount alice: %v", err)
	}
	if err := k.SetAccount("bob", "pb"); err != nil {
		t.Fatalf("SetAccount bob: %v", err)
	}

	// Removing one account must leave the other intact.
	if err := k.RemoveAccount("alice"); err != nil {
		t.Fatalf("RemoveAccount alice: %v", err)
	}
	names, err := k.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(names) != 1 || names[0] != "bob" {
		t.Fatalf("accounts = %v, want [bob]", names)
	}
	if pass, err := k.GetAccount("bob"); err != nil || pass != "pb" {
		t.Fatalf("GetAccount bob = %q,%v want pb,nil", pass, err)
	}
}

func TestMockCredentialStore_SetAndGetAccount(t *testing.T) {
	store := &MockCredentialStore{}

	err := store.SetAccount("alice", "password123")
	if err != nil {
		t.Fatalf("SetAccount failed: %v", err)
	}

	pass, err := store.GetAccount("alice")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if pass != "password123" {
		t.Errorf("expected password 'password123', got %q", pass)
	}
}

func TestMockCredentialStore_GetBeforeSet(t *testing.T) {
	store := &MockCredentialStore{}

	_, err := store.GetAccount("nobody")
	if err != ErrNoCredentials {
		t.Errorf("expected ErrNoCredentials, got %v", err)
	}
}

func TestMockCredentialStore_MultipleAccounts(t *testing.T) {
	store := &MockCredentialStore{}

	// Add two accounts.
	store.SetAccount("alice", "pass_alice")
	store.SetAccount("bob", "pass_bob")

	// List should return both, sorted alphabetically.
	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
	if accounts[0] != "alice" {
		t.Errorf("expected first account 'alice', got %q", accounts[0])
	}
	if accounts[1] != "bob" {
		t.Errorf("expected second account 'bob', got %q", accounts[1])
	}

	// Get each by username.
	pass, err := store.GetAccount("alice")
	if err != nil || pass != "pass_alice" {
		t.Errorf("expected pass_alice, got %q (err=%v)", pass, err)
	}
	pass, err = store.GetAccount("bob")
	if err != nil || pass != "pass_bob" {
		t.Errorf("expected pass_bob, got %q (err=%v)", pass, err)
	}

	// Remove one.
	err = store.RemoveAccount("alice")
	if err != nil {
		t.Fatalf("RemoveAccount failed: %v", err)
	}

	// Verify list has only bob.
	accounts, err = store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0] != "bob" {
		t.Errorf("expected 'bob', got %q", accounts[0])
	}

	// alice should be gone.
	_, err = store.GetAccount("alice")
	if err != ErrNoCredentials {
		t.Errorf("expected ErrNoCredentials for removed account, got %v", err)
	}
}

func TestMockCredentialStore_ListEmpty(t *testing.T) {
	store := &MockCredentialStore{}

	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("expected empty list, got %d accounts", len(accounts))
	}
}

func TestMockCredentialStore_OverwriteAccount(t *testing.T) {
	store := &MockCredentialStore{}

	store.SetAccount("alice", "pass1")
	store.SetAccount("alice", "pass2")

	pass, err := store.GetAccount("alice")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if pass != "pass2" {
		t.Errorf("expected 'pass2', got %q", pass)
	}

	// Should still only have one entry.
	accounts, _ := store.ListAccounts()
	if len(accounts) != 1 {
		t.Errorf("expected 1 account after overwrite, got %d", len(accounts))
	}
}

func TestMockCredentialStore_RemoveNonexistent(t *testing.T) {
	store := &MockCredentialStore{}

	// Removing a nonexistent account should not error.
	err := store.RemoveAccount("nobody")
	if err != nil {
		t.Errorf("RemoveAccount on nonexistent should not error, got %v", err)
	}
}

func TestMockCredentialStore_RoundTrip(t *testing.T) {
	store := &MockCredentialStore{}

	// Set -> Get -> Remove -> Get (error) -> Set -> Get
	store.SetAccount("alice", "pass1")

	p, err := store.GetAccount("alice")
	if err != nil || p != "pass1" {
		t.Fatalf("first GetAccount failed: p=%q err=%v", p, err)
	}

	store.RemoveAccount("alice")

	_, err = store.GetAccount("alice")
	if err != ErrNoCredentials {
		t.Fatalf("expected ErrNoCredentials after Remove, got %v", err)
	}

	store.SetAccount("bob", "pass2")

	p, err = store.GetAccount("bob")
	if err != nil || p != "pass2" {
		t.Fatalf("second GetAccount failed: p=%q err=%v", p, err)
	}
}

// Verify that both KeyringStore and MockCredentialStore implement CredentialStore.
func TestCredentialStore_InterfaceCompliance(t *testing.T) {
	var _ CredentialStore = &KeyringStore{}
	var _ CredentialStore = &MockCredentialStore{}
}
