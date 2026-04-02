package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// TokenStore manages persistence of OAuth2 tokens.
type TokenStore interface {
	Save(ctx context.Context, accountID string, token *oauth2.Token) error
	Load(ctx context.Context, accountID string) (*oauth2.Token, error)
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

// validateAccountID rejects account IDs that could escape the token directory.
func validateAccountID(accountID string) error {
	if accountID == "" {
		return fmt.Errorf("empty account ID")
	}
	if strings.ContainsAny(accountID, `/\`) {
		return fmt.Errorf("invalid account ID: contains path separator")
	}
	cleaned := filepath.Base(filepath.Clean(accountID))
	if cleaned != accountID || cleaned == ".." || cleaned == "." {
		return fmt.Errorf("invalid account ID: %q", accountID)
	}
	return nil
}

func (s *JSONTokenStore) tokenPath(accountID string) string {
	return filepath.Join(s.dir, accountID+".json")
}

// Save persists a token for the given account.
func (s *JSONTokenStore) Save(_ context.Context, accountID string, token *oauth2.Token) error {
	if err := validateAccountID(accountID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.tokenPath(accountID), data, 0600)
}

// Load reads a stored token for the given account.
func (s *JSONTokenStore) Load(_ context.Context, accountID string) (*oauth2.Token, error) {
	if err := validateAccountID(accountID); err != nil {
		return nil, err
	}

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
	if err := validateAccountID(accountID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.tokenPath(accountID))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
