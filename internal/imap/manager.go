package imap

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Araozu/go-mail/internal/auth"
	"github.com/Araozu/go-mail/internal/domain"
)

// ErrAccountExists is returned when trying to register an account that is
// already registered with the Manager.
var ErrAccountExists = errors.New("account already registered")

// imapHost maps provider names to their IMAP server addresses.
var imapHosts = map[string]string{
	"gmail":   "imap.gmail.com:993",
	"outlook": "outlook.office365.com:993",
	"yahoo":   "imap.mail.yahoo.com:993",
}

// clientEntry wraps a Client with a per-account mutex so callers
// serialise access through DoWithClient.
type clientEntry struct {
	client *Client
	mu     sync.Mutex
}

// Manager manages IMAP clients for multiple accounts.
// Each account gets a single Client. Access is serialized per account
// via DoWithClient.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*clientEntry
}

// NewManager creates a new multi-account IMAP manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*clientEntry),
	}
}

// AddAccount registers an account and creates an IMAP client for it.
// It does NOT connect automatically — call DoWithClient + Connect.
func (m *Manager) AddAccount(account *domain.Account, provider auth.Provider) error {
	if account == nil {
		return fmt.Errorf("nil account")
	}
	if provider == nil {
		return fmt.Errorf("nil provider")
	}
	if provider.Name() != account.Provider {
		return fmt.Errorf("provider mismatch: account has %q but provider is %q", account.Provider, provider.Name())
	}

	host, ok := imapHosts[account.Provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", account.Provider)
	}
	if host == "" {
		return fmt.Errorf("empty IMAP host for provider: %s", account.Provider)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[account.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAccountExists, account.ID)
	}

	m.clients[account.ID] = &clientEntry{
		client: NewClient(account, provider, host),
	}
	return nil
}

// AddAccountWithHost registers an account with a custom IMAP host.
func (m *Manager) AddAccountWithHost(account *domain.Account, provider auth.Provider, host string) error {
	if account == nil {
		return fmt.Errorf("nil account")
	}
	if host == "" {
		return fmt.Errorf("empty IMAP host")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[account.ID]; exists {
		return fmt.Errorf("%w: %s", ErrAccountExists, account.ID)
	}

	m.clients[account.ID] = &clientEntry{
		client: NewClient(account, provider, host),
	}
	return nil
}

// RemoveAccount disconnects and removes a client.
// Disconnect is performed outside the lock to avoid blocking.
func (m *Manager) RemoveAccount(accountID string) error {
	m.mu.Lock()
	entry, ok := m.clients[accountID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("account %s not found", accountID)
	}
	delete(m.clients, accountID)
	m.mu.Unlock()

	// Disconnect outside the lock — network I/O can be slow.
	entry.client.Disconnect()
	return nil
}

// DoWithClient looks up the client for accountID and calls fn while
// holding a per-account lock, serialising all operations on the same
// IMAP connection.
func (m *Manager) DoWithClient(accountID string, fn func(*Client) error) error {
	m.mu.RLock()
	entry, ok := m.clients[accountID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	return fn(entry.client)
}

// ConnectClient connects the IMAP client for the given account.
func (m *Manager) ConnectClient(ctx context.Context, accountID string) error {
	return m.DoWithClient(accountID, func(c *Client) error {
		return c.Connect(ctx)
	})
}

// DisconnectAll disconnects all clients.
// Disconnects are performed outside the manager lock.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	entries := make([]*clientEntry, 0, len(m.clients))
	for _, entry := range m.clients {
		entries = append(entries, entry)
	}
	m.mu.Unlock()

	for _, entry := range entries {
		entry.client.Disconnect()
	}
}
