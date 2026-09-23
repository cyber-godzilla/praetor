package session

import "sort"

type MockCredentialStore struct {
	accounts map[string]string
}

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

func (m *MockCredentialStore) GetAccount(username string) (string, error) {
	if m.accounts == nil {
		return "", ErrNoCredentials
	}
	password, ok := m.accounts[username]
	if !ok {
		return "", ErrNoCredentials
	}
	return password, nil
}

func (m *MockCredentialStore) SetAccount(username, password string) error {
	if m.accounts == nil {
		m.accounts = make(map[string]string)
	}
	m.accounts[username] = password
	return nil
}

func (m *MockCredentialStore) RemoveAccount(username string) error {
	if m.accounts != nil {
		delete(m.accounts, username)
	}
	return nil
}
