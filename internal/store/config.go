package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/Araozu/go-mail/internal/domain"
)

// ConfigStore manages persistence of account configurations.
type ConfigStore interface {
	SaveAccount(account *domain.Account) error
	LoadAccounts() ([]*domain.Account, error)
	DeleteAccount(id string) error
}

// JSONConfigStore implements ConfigStore using a JSON file on disk.
type JSONConfigStore struct {
	dir string
	mu  sync.RWMutex
}

// NewJSONConfigStore creates a new store at the given directory.
// It creates the directory if it doesn't exist.
func NewJSONConfigStore(dir string) (*JSONConfigStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &JSONConfigStore{dir: dir}, nil
}

func (s *JSONConfigStore) accountsPath() string {
	return filepath.Join(s.dir, "accounts.json")
}

// SaveAccount adds or updates an account in the store.
func (s *JSONConfigStore) SaveAccount(account *domain.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.loadAccountsLocked()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Update existing or append
	found := false
	for i, a := range accounts {
		if a.ID == account.ID {
			accounts[i] = account
			found = true
			break
		}
	}
	if !found {
		accounts = append(accounts, account)
	}

	return s.writeAccounts(accounts)
}

// LoadAccounts returns all stored accounts.
func (s *JSONConfigStore) LoadAccounts() ([]*domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadAccountsLocked()
}

// DeleteAccount removes an account by ID.
func (s *JSONConfigStore) DeleteAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.loadAccountsLocked()
	if err != nil {
		return err
	}

	filtered := make([]*domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if a.ID != id {
			filtered = append(filtered, a)
		}
	}

	return s.writeAccounts(filtered)
}

func (s *JSONConfigStore) loadAccountsLocked() ([]*domain.Account, error) {
	data, err := os.ReadFile(s.accountsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var accounts []*domain.Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *JSONConfigStore) writeAccounts(accounts []*domain.Account) error {
	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.accountsPath(), data, 0600)
}
