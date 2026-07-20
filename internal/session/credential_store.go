package session

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	CredentialBackendKeyring       = "keyring"
	CredentialBackendEncryptedFile = "encrypted_file"
	CredentialBackendDisabled      = "disabled"
	defaultCredentialKeyEnv        = "PRAETOR_CREDENTIALS_KEY"
)

var ErrCredentialStorageDisabled = errors.New("secure credential storage is disabled")

// CredentialStoreOptions are resolved by each executable from its normal
// configuration and state directories. Encrypted-file keys are read exactly
// once from KeyEnv and then removed from the process environment.
type CredentialStoreOptions struct {
	Backend  string
	StateDir string
	FilePath string
	KeyEnv   string
}

// NewCredentialStore constructs the explicitly selected backend. It never
// falls back to another backend: unavailable keyrings remain visible at
// runtime, while an invalid encrypted-file configuration fails startup.
func NewCredentialStore(options CredentialStoreOptions) (CredentialStore, error) {
	switch options.Backend {
	case CredentialBackendKeyring:
		return &KeyringStore{}, nil
	case CredentialBackendDisabled:
		return &DisabledCredentialStore{}, nil
	case CredentialBackendEncryptedFile:
		keyEnv := options.KeyEnv
		if keyEnv == "" {
			keyEnv = defaultCredentialKeyEnv
		}
		encodedKey, ok := os.LookupEnv(keyEnv)
		if !ok || encodedKey == "" {
			return nil, fmt.Errorf("credentials backend encrypted_file requires %s", keyEnv)
		}
		// The environment is only a startup delivery mechanism. Removing it does
		// not erase parent-process or kernel copies, but avoids carrying it into
		// child processes and reduces accidental exposure through later dumps.
		_ = os.Unsetenv(keyEnv)
		key, err := decodeCredentialKey(encodedKey)
		encodedKey = ""
		if err != nil {
			return nil, fmt.Errorf("credentials backend encrypted_file: %w", err)
		}
		path := options.FilePath
		if path == "" {
			path = filepath.Join(options.StateDir, "credentials", "credentials.enc")
		}
		store, err := NewEncryptedFileCredentialStore(path, key)
		clear(key)
		if err != nil {
			return nil, fmt.Errorf("credentials backend encrypted_file: %w", err)
		}
		if _, err := store.ListAccounts(); err != nil {
			return nil, fmt.Errorf("credentials backend encrypted_file: %w", err)
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported credentials backend %q", options.Backend)
	}
}

func decodeCredentialKey(encoded string) ([]byte, error) {
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, decoder := range decoders {
		key, err := decoder.DecodeString(encoded)
		if err == nil {
			if len(key) != 32 {
				clear(key)
				return nil, fmt.Errorf("credential encryption key must decode to exactly 32 bytes")
			}
			return key, nil
		}
	}
	return nil, fmt.Errorf("credential encryption key must be base64 encoded")
}

// DisabledCredentialStore makes the no-persistence policy explicit while
// preserving ordinary username/password login.
type DisabledCredentialStore struct{}

func (d *DisabledCredentialStore) Descriptor() CredentialStoreDescriptor {
	return CredentialStoreDescriptor{Backend: CredentialBackendDisabled, CanStore: false}
}

func (d *DisabledCredentialStore) ListAccounts() ([]string, error) {
	return []string{}, nil
}

func (d *DisabledCredentialStore) GetAccount(string) (string, error) {
	return "", ErrCredentialStorageDisabled
}

func (d *DisabledCredentialStore) SetAccount(string, string) error {
	return ErrCredentialStorageDisabled
}

func (d *DisabledCredentialStore) RemoveAccount(string) error {
	return ErrCredentialStorageDisabled
}
