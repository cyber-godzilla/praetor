package session

import (
	"encoding/json"
	"sort"

	"github.com/zalando/go-keyring"
)

const (
	keyringService    = "praetor"
	keyringAccountKey = "accounts"
)

// Account holds a username and password pair.
type Account struct {
	Username string
	Password string
}

// CredentialStore defines the interface for storing and retrieving
// multiple user accounts (username/password pairs).
type CredentialStore interface {
	ListAccounts() ([]string, error)
	GetAccount(username string) (string, error)
	SetAccount(username, password string) error
	RemoveAccount(username string) error
}

// ErrNoCredentials is returned when no credentials are stored.
var ErrNoCredentials = keyring.ErrNotFound

// KeyringStore uses the system keyring (via zalando/go-keyring) to
// persist credentials securely. All accounts are stored as a single
// JSON-encoded map[string]string under the key "accounts".
type KeyringStore struct{}

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
		return make(map[string]string), nil
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
	accounts, err := k.loadAccounts()
	if err != nil {
		return err
	}
	accounts[username] = password
	return k.saveAccounts(accounts)
}

// RemoveAccount removes the given username from the store.
func (k *KeyringStore) RemoveAccount(username string) error {
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
}

// ListAccounts returns stored usernames sorted alphabetically.
func (m *MockCredentialStore) ListAccounts() ([]string, error) {
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
	if m.accounts == nil {
		m.accounts = make(map[string]string)
	}
	m.accounts[username] = password
	return nil
}

// RemoveAccount removes the given username from the store.
func (m *MockCredentialStore) RemoveAccount(username string) error {
	if m.accounts != nil {
		delete(m.accounts, username)
	}
	return nil
}
