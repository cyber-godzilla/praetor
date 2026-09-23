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
		// A corrupt/truncated blob must NOT read as "no accounts stored" — the
		// ordinary SetAccount would then overwrite the entry and could destroy a
		// merely-misread blob. Surface the error so read and write paths refuse to
		// modify the unreadable entry.
		return nil, fmt.Errorf("keyring blob corrupt: %w", err)
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
	keyringMu.Lock()
	defer keyringMu.Unlock()
	accounts, err := k.loadAccounts()
	if err != nil {
		return err
	}
	accounts[username] = password
	return k.saveAccounts(accounts)
}

// RemoveAccount removes the given username from the store.
func (k *KeyringStore) RemoveAccount(username string) error {
	keyringMu.Lock()
	defer keyringMu.Unlock()
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
