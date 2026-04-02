package imap

import (
	"fmt"
	"sync"

	"github.com/Araozu/go-mail/internal/auth"
	"github.com/Araozu/go-mail/internal/domain"
)

// imapHost maps provider names to their IMAP server addresses.
var imapHosts = map[string]string{
	"gmail":   "imap.gmail.com:993",
	"outlook": "outlook.office365.com:993",
	"yahoo":   "imap.mail.yahoo.com:993",
}

// Manager manages IMAP clients for multiple accounts.
// Each account gets a single Client. Access is serialized per account.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

// NewManager creates a new multi-account IMAP manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// AddAccount registers an account and creates an IMAP client for it.
// It does NOT connect automatically — call Connect on the returned client.
func (m *Manager) AddAccount(account *domain.Account, provider auth.Provider) error {
	host, ok := imapHosts[account.Provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", account.Provider)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[account.ID]; exists {
		return fmt.Errorf("account %s already registered", account.ID)
	}

	m.clients[account.ID] = NewClient(account, provider, host)
	return nil
}

// AddAccountWithHost registers an account with a custom IMAP host.
func (m *Manager) AddAccountWithHost(account *domain.Account, provider auth.Provider, host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[account.ID]; exists {
		return fmt.Errorf("account %s already registered", account.ID)
	}

	m.clients[account.ID] = NewClient(account, provider, host)
	return nil
}

// RemoveAccount disconnects and removes a client.
func (m *Manager) RemoveAccount(accountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.clients[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}

	c.Disconnect()
	delete(m.clients, accountID)
	return nil
}

// GetClient returns the IMAP client for an account.
func (m *Manager) GetClient(accountID string) (*Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.clients[accountID]
	if !ok {
		return nil, fmt.Errorf("account %s not found", accountID)
	}
	return c, nil
}

// DisconnectAll disconnects all clients.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.clients {
		c.Disconnect()
	}
}
