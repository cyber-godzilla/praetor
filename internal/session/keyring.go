package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/zalando/go-keyring"
)

// keyringMu serializes the read-modify-write cycle in SetAccount/RemoveAccount
// so two concurrent writers can't lose each other's update.
var keyringMu sync.Mutex

const (
	keyringService    = "praetor"
	keyringAccountKey = "accounts"
)

// CredentialStore defines the interface for storing and retrieving
// multiple user accounts (username/password pairs).
type CredentialStore interface {
	Descriptor() CredentialStoreDescriptor
	ListAccounts() ([]string, error)
	GetAccount(username string) (string, error)
	SetAccount(username, password string) error
	RepairAccounts(username, password string) error
	RemoveAccount(username string) error
}

// CredentialStoreDescriptor contains non-secret, static backend capabilities.
// Runtime availability is determined by attempting ListAccounts so a missing
// or locked keyring is reported instead of being mistaken for an empty store.
type CredentialStoreDescriptor struct {
	Backend  string
	CanStore bool
}

// ErrNoCredentials is returned when no credentials are stored.
var ErrNoCredentials = keyring.ErrNotFound

// KeyringStore uses the system keyring (via zalando/go-keyring) to
// persist credentials securely. All accounts are stored as a single
// JSON-encoded map[string]string under the key "accounts".
type KeyringStore struct {
	mu sync.Mutex
}

func (k *KeyringStore) Descriptor() CredentialStoreDescriptor {
	return CredentialStoreDescriptor{Backend: "keyring", CanStore: true}
}

// loadAccounts reads the JSON map from the keyring.
func (k *KeyringStore) loadAccounts() (map[string]string, error) {
	raw, err := keyring.Get(keyringService, keyringAccountKey)
	if err != nil {
		if err == keyring.ErrNotFound {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var accounts map[string]string
	if err := json.Unmarshal([]byte(raw), &accounts); err != nil {
		// A corrupt/truncated blob must NOT read as "no accounts stored" — the
		// ordinary SetAccount would then overwrite the entry and could destroy a
		// merely-misread blob. Surface the error so read and write paths refuse to
		// modify the unreadable entry.
		return nil, fmt.Errorf("keyring blob corrupt: %w", err)
	}
	if accounts == nil {
		accounts = make(map[string]string)
	}
	return accounts, nil
}

// saveAccounts writes the JSON map to the keyring.
func (k *KeyringStore) saveAccounts(accounts map[string]string) error {
	data, err := json.Marshal(accounts)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, keyringAccountKey, string(data))
}

// ListAccounts returns stored usernames sorted alphabetically.
func (k *KeyringStore) ListAccounts() ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	accounts, err := k.loadAccounts()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(accounts))
	for name := range accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetAccount returns the password for the given username.
func (k *KeyringStore) GetAccount(username string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	accounts, err := k.loadAccounts()
	if err != nil {
		return "", err
	}
	pass, ok := accounts[username]
	if !ok {
		return "", ErrNoCredentials
	}
	return pass, nil
}

// SetAccount stores the username and password.
func (k *KeyringStore) SetAccount(username, password string) error {
	keyringMu.Lock()
	defer keyringMu.Unlock()
	k.mu.Lock()
	defer k.mu.Unlock()
	accounts, err := k.loadAccounts()
	if err != nil {
		return err
	}
	accounts[username] = password
	return k.saveAccounts(accounts)
}

// RepairAccounts explicitly replaces an unreadable/corrupt account blob with
// a new single-account store after the user has chosen recovery.
func (k *KeyringStore) RepairAccounts(username, password string) error {
	keyringMu.Lock()
	defer keyringMu.Unlock()
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.saveAccounts(map[string]string{username: password})
}

// RemoveAccount removes the given username from the store.
func (k *KeyringStore) RemoveAccount(username string) error {
	keyringMu.Lock()
	defer keyringMu.Unlock()
	k.mu.Lock()
	defer k.mu.Unlock()
	accounts, err := k.loadAccounts()
	if err != nil {
		return err
	}
	delete(accounts, username)
	if len(accounts) == 0 {
		// Clean up the keyring entry entirely.
		return keyring.Delete(keyringService, keyringAccountKey)
	}
	return k.saveAccounts(accounts)
}

// MockCredentialStore is an in-memory credential store for testing.
type MockCredentialStore struct {
	accounts map[string]string
	Err      error
}

func (m *MockCredentialStore) Descriptor() CredentialStoreDescriptor {
	return CredentialStoreDescriptor{Backend: "memory", CanStore: true}
}

// ListAccounts returns stored usernames sorted alphabetically.
func (m *MockCredentialStore) ListAccounts() ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.accounts == nil {
		return nil, nil
	}
	names := make([]string, 0, len(m.accounts))
	for name := range m.accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetAccount returns the password for the given username.
func (m *MockCredentialStore) GetAccount(username string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if m.accounts == nil {
		return "", ErrNoCredentials
	}
	pass, ok := m.accounts[username]
	if !ok {
		return "", ErrNoCredentials
	}
	return pass, nil
}

// SetAccount stores the username and password.
func (m *MockCredentialStore) SetAccount(username, password string) error {
	if m.Err != nil {
		return m.Err
	}
	if m.accounts == nil {
		m.accounts = make(map[string]string)
	}
	m.accounts[username] = password
	return nil
}

// RepairAccounts overwrites the mock store with a single account.
func (m *MockCredentialStore) RepairAccounts(username, password string) error {
	m.accounts = map[string]string{username: password}
	return nil
}

// RemoveAccount removes the given username from the store.
func (m *MockCredentialStore) RemoveAccount(username string) error {
	if m.Err != nil {
		return m.Err
	}
	if m.accounts != nil {
		delete(m.accounts, username)
	}
	return nil
}
