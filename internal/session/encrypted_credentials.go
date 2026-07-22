package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	encryptedCredentialVersion   = 1
	encryptedCredentialAlgorithm = "AES-256-GCM"
	maxCredentialFileSize        = 1 << 20
)

var credentialAdditionalData = []byte("praetor credentials v1")

type encryptedCredentialEnvelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// EncryptedFileCredentialStore stores the complete account map in one
// authenticated-encryption envelope. The master key is retained only in
// process memory and is never written beside the file.
type EncryptedFileCredentialStore struct {
	mu   sync.Mutex
	path string
	aead cipher.AEAD
}

func NewEncryptedFileCredentialStore(path string, key []byte) (*EncryptedFileCredentialStore, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("credential encryption key must be 32 bytes")
	}
	if path == "" {
		return nil, fmt.Errorf("credential file path is required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating credential cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating credential AEAD: %w", err)
	}
	return &EncryptedFileCredentialStore{path: filepath.Clean(path), aead: aead}, nil
}

func (s *EncryptedFileCredentialStore) Descriptor() CredentialStoreDescriptor {
	return CredentialStoreDescriptor{Backend: CredentialBackendEncryptedFile, CanStore: true}
}

func (s *EncryptedFileCredentialStore) ListAccounts() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts, err := s.loadAccountsLocked()
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

func (s *EncryptedFileCredentialStore) GetAccount(username string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts, err := s.loadAccountsLocked()
	if err != nil {
		return "", err
	}
	password, ok := accounts[username]
	if !ok {
		return "", ErrNoCredentials
	}
	return password, nil
}

func (s *EncryptedFileCredentialStore) SetAccount(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts, err := s.loadAccountsLocked()
	if err != nil {
		return err
	}
	accounts[username] = password
	return s.saveAccountsLocked(accounts)
}

// RepairAccounts replaces the encrypted file with a new single-account store
// without first reading the existing contents. This is the explicit recovery
// path for a store that became corrupt or unreadable after startup; ordinary
// SetAccount deliberately refuses to overwrite unreadable data.
func (s *EncryptedFileCredentialStore) RepairAccounts(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveAccountsLocked(map[string]string{username: password})
}

func (s *EncryptedFileCredentialStore) RemoveAccount(username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts, err := s.loadAccountsLocked()
	if err != nil {
		return err
	}
	delete(accounts, username)
	if len(accounts) == 0 {
		err := os.Remove(s.path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("removing encrypted credential store: %w", err)
		}
		return syncDirectory(filepath.Dir(s.path))
	}
	return s.saveAccountsLocked(accounts)
}

func (s *EncryptedFileCredentialStore) loadAccountsLocked() (map[string]string, error) {
	info, err := os.Lstat(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("examining encrypted credential store: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("encrypted credential store is not a regular file")
	}
	if info.Size() > maxCredentialFileSize {
		return nil, fmt.Errorf("encrypted credential store exceeds %d bytes", maxCredentialFileSize)
	}
	file, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("opening encrypted credential store: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCredentialFileSize+1))
	closeErr := file.Close()
	if err != nil {
		return nil, fmt.Errorf("reading encrypted credential store: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("closing encrypted credential store: %w", closeErr)
	}
	if len(data) > maxCredentialFileSize {
		return nil, fmt.Errorf("encrypted credential store exceeds %d bytes", maxCredentialFileSize)
	}
	defer clear(data)

	var envelope encryptedCredentialEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding encrypted credential store: %w", err)
	}
	if envelope.Version != encryptedCredentialVersion || envelope.Algorithm != encryptedCredentialAlgorithm {
		return nil, fmt.Errorf("unsupported encrypted credential store format version=%d algorithm=%q", envelope.Version, envelope.Algorithm)
	}
	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil || len(nonce) != s.aead.NonceSize() {
		clear(nonce)
		return nil, fmt.Errorf("encrypted credential store has an invalid nonce")
	}
	defer clear(nonce)
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("encrypted credential store has invalid ciphertext encoding")
	}
	defer clear(ciphertext)
	plaintext, err := s.aead.Open(nil, nonce, ciphertext, credentialAdditionalData)
	if err != nil {
		return nil, fmt.Errorf("decrypting encrypted credential store: authentication failed")
	}
	defer clear(plaintext)
	var accounts map[string]string
	if err := json.Unmarshal(plaintext, &accounts); err != nil {
		return nil, fmt.Errorf("decoding credential account data: %w", err)
	}
	if accounts == nil {
		accounts = make(map[string]string)
	}
	return accounts, nil
}

func (s *EncryptedFileCredentialStore) saveAccountsLocked(accounts map[string]string) error {
	plaintext, err := json.Marshal(accounts)
	if err != nil {
		return fmt.Errorf("encoding credential account data: %w", err)
	}
	defer clear(plaintext)
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generating credential nonce: %w", err)
	}
	defer clear(nonce)
	ciphertext := s.aead.Seal(nil, nonce, plaintext, credentialAdditionalData)
	defer clear(ciphertext)
	envelope := encryptedCredentialEnvelope{
		Version:    encryptedCredentialVersion,
		Algorithm:  encryptedCredentialAlgorithm,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encoding encrypted credential store: %w", err)
	}
	defer clear(data)
	if len(data) > maxCredentialFileSize {
		return fmt.Errorf("encrypted credential store exceeds %d bytes", maxCredentialFileSize)
	}
	return atomicWriteCredentialFile(s.path, data)
}

func atomicWriteCredentialFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating credential directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("creating credential temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return fmt.Errorf("securing credential temporary file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("writing credential temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("syncing credential temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing credential temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("installing encrypted credential store: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing encrypted credential store: %w", err)
	}
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("opening credential directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("syncing credential directory: %w", err)
	}
	return nil
}
