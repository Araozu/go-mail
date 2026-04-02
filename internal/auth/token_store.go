package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
)

// TokenStore manages persistence of OAuth2 tokens.
type TokenStore interface {
	Save(accountID string, token *oauth2.Token) error
	Load(accountID string) (*oauth2.Token, error)
	Delete(accountID string) error
}

// JSONTokenStore implements TokenStore using individual JSON files per account.
type JSONTokenStore struct {
	dir string
	mu  sync.RWMutex
}

// NewJSONTokenStore creates a new token store at the given directory.
func NewJSONTokenStore(dir string) (*JSONTokenStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &JSONTokenStore{dir: dir}, nil
}

func (s *JSONTokenStore) tokenPath(accountID string) string {
	return filepath.Join(s.dir, accountID+".json")
}

// Save persists a token for the given account.
func (s *JSONTokenStore) Save(accountID string, token *oauth2.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.tokenPath(accountID), data, 0600)
}

// Load reads a stored token for the given account.
func (s *JSONTokenStore) Load(accountID string) (*oauth2.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.tokenPath(accountID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("no token found for account: " + accountID)
		}
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// Delete removes a stored token for the given account.
func (s *JSONTokenStore) Delete(accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.tokenPath(accountID))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
