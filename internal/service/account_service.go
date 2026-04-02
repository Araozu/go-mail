package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Araozu/go-mail/internal/auth"
	"github.com/Araozu/go-mail/internal/domain"
	imapPkg "github.com/Araozu/go-mail/internal/imap"
	"github.com/Araozu/go-mail/internal/store"
)

// AccountService manages account lifecycle: add, remove, list, connect.
type AccountService struct {
	store   store.ConfigStore
	manager *imapPkg.Manager
	// providers maps provider name ("gmail") to its auth provider
	providers map[string]auth.Provider
}

// NewAccountService creates a new account service.
func NewAccountService(configStore store.ConfigStore, manager *imapPkg.Manager) *AccountService {
	return &AccountService{
		store:     configStore,
		manager:   manager,
		providers: make(map[string]auth.Provider),
	}
}

// RegisterProvider adds an auth provider for a given provider name.
func (s *AccountService) RegisterProvider(name string, provider auth.Provider) {
	s.providers[name] = provider
}

// StartAddAccount begins the OAuth flow for a new account and returns
// the authorization URL the user must visit.
func (s *AccountService) StartAddAccount(providerName string) (authURL string, err error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}
	return provider.StartFlow()
}

// CompleteAddAccount finishes the OAuth flow with the auth code and
// creates a new account. The state parameter must match the state from
// the OAuth callback. Returns the created account.
func (s *AccountService) CompleteAddAccount(ctx context.Context, providerName, email, code, state string) (*domain.Account, error) {
	provider, ok := s.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating ID: %w", err)
	}

	account := &domain.Account{
		ID:       id,
		Email:    email,
		Provider: providerName,
		AuthType: "oauth2",
	}

	// Exchange code and save token
	if err := provider.CompleteFlowWithAccount(ctx, state, code, account.ID); err != nil {
		return nil, fmt.Errorf("completing auth flow: %w", err)
	}

	// Persist account config
	if err := s.store.SaveAccount(account); err != nil {
		return nil, fmt.Errorf("saving account: %w", err)
	}

	// Register with IMAP manager
	if err := s.manager.AddAccount(account, provider); err != nil {
		return nil, fmt.Errorf("registering IMAP client: %w", err)
	}

	return account, nil
}

// RemoveAccount removes an account and its IMAP connection.
func (s *AccountService) RemoveAccount(id string) error {
	s.manager.RemoveAccount(id)
	return s.store.DeleteAccount(id)
}

// ListAccounts returns all configured accounts.
func (s *AccountService) ListAccounts() ([]*domain.Account, error) {
	return s.store.LoadAccounts()
}

// ConnectAll loads all stored accounts and connects them.
func (s *AccountService) ConnectAll(ctx context.Context) error {
	accounts, err := s.store.LoadAccounts()
	if err != nil {
		return fmt.Errorf("loading accounts: %w", err)
	}

	var firstErr error
	for _, account := range accounts {
		provider, ok := s.providers[account.Provider]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("no provider for %s (account %s)", account.Provider, account.Email)
			}
			continue
		}

		if err := s.manager.AddAccount(account, provider); err != nil {
			if !errors.Is(err, imapPkg.ErrAccountExists) {
				// Real failure — record and skip this account.
				if firstErr == nil {
					firstErr = fmt.Errorf("registering %s: %w", account.Email, err)
				}
				continue
			}
			// Already registered — fall through to connect.
		}

		if err := s.manager.ConnectClient(ctx, account.ID); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("connecting %s: %w", account.Email, err)
			}
			continue
		}
	}

	return firstErr
}

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
