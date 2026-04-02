package mailclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Araozu/go-mail/internal/auth"
	"github.com/Araozu/go-mail/internal/domain"
	imapPkg "github.com/Araozu/go-mail/internal/imap"
	"github.com/Araozu/go-mail/internal/service"
	"github.com/Araozu/go-mail/internal/store"
)

// Client is the public API for the mail client.
// All consumers (CLI, UI, Wails) interact through this.
type Client struct {
	accounts  *service.AccountService
	mailboxes *service.MailboxService
	messages  *service.MessageService
	manager   *imapPkg.Manager
	events    chan Event
	closeOnce sync.Once
	closed    atomic.Bool
}

// New creates a new mail client with the given options.
func New(opts ...Option) (*Client, error) {
	options := defaultOptions()
	for _, o := range opts {
		o(options)
	}

	// Resolve config dir
	configDir := options.ConfigDir
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home dir: %w", err)
		}
		configDir = filepath.Join(home, ".config", "go-mail")
	}

	// Initialize store
	configStore, err := store.NewJSONConfigStore(configDir)
	if err != nil {
		return nil, fmt.Errorf("creating config store: %w", err)
	}

	// Initialize token store
	tokenDir := filepath.Join(configDir, "tokens")
	tokenStore, err := auth.NewJSONTokenStore(tokenDir)
	if err != nil {
		return nil, fmt.Errorf("creating token store: %w", err)
	}

	// Initialize IMAP manager
	manager := imapPkg.NewManager()

	// Initialize account service
	accountSvc := service.NewAccountService(configStore, manager)

	// Register Gmail provider if credentials are provided
	if options.GmailClientID != "" && options.GmailClientSecret != "" {
		gmailProvider := auth.NewGmailProvider(
			options.GmailClientID,
			options.GmailClientSecret,
			options.GmailRedirectURL,
			tokenStore,
		)
		accountSvc.RegisterProvider("gmail", gmailProvider)
	}

	mailboxSvc, err := service.NewMailboxService(manager)
	if err != nil {
		return nil, fmt.Errorf("creating mailbox service: %w", err)
	}

	return &Client{
		accounts:  accountSvc,
		mailboxes: mailboxSvc,
		messages:  service.NewMessageService(manager),
		manager:   manager,
		events:    make(chan Event, options.EventBufferSize),
	}, nil
}

// --- Account operations ---

// StartAddAccount begins the OAuth flow for a new account.
// Returns the authorization URL the user must visit.
func (c *Client) StartAddAccount(provider string) (string, error) {
	return c.accounts.StartAddAccount(provider)
}

// CompleteAddAccount finishes the OAuth flow with the auth code and state.
// After registering the account, it attempts to connect the IMAP session
// and only emits EventConnected if the connection succeeds.
func (c *Client) CompleteAddAccount(ctx context.Context, provider, email, code, state string) (*domain.Account, error) {
	account, err := c.accounts.CompleteAddAccount(ctx, provider, email, code, state)
	if err != nil {
		return nil, err
	}

	// Attempt to connect the newly registered account.
	if err := c.manager.ConnectClient(ctx, account.ID); err != nil {
		// Account is registered but not connected.
		return account, fmt.Errorf("account added but connection failed: %w", err)
	}

	c.emit(Event{
		Type:      EventConnected,
		AccountID: account.ID,
	})

	return account, nil
}

// RemoveAccount removes an account and disconnects it.
func (c *Client) RemoveAccount(id string) error {
	err := c.accounts.RemoveAccount(id)
	if err == nil {
		c.emit(Event{
			Type:      EventDisconnected,
			AccountID: id,
		})
	}
	return err
}

// ListAccounts returns all configured accounts.
func (c *Client) ListAccounts() ([]*domain.Account, error) {
	return c.accounts.ListAccounts()
}

// ConnectAll loads stored accounts and connects them all.
func (c *Client) ConnectAll(ctx context.Context) error {
	return c.accounts.ConnectAll(ctx)
}

// --- Mailbox operations ---

// ListMailboxes returns all mailboxes for the given account.
func (c *Client) ListMailboxes(accountID string) ([]*domain.Mailbox, error) {
	return c.mailboxes.ListMailboxes(accountID)
}

// --- Message operations ---

// ListMessages fetches the most recent messages from a mailbox.
func (c *Client) ListMessages(accountID, mailbox string, limit uint32) ([]*domain.Envelope, error) {
	return c.messages.ListMessages(accountID, mailbox, limit)
}

// GetMessage fetches a full message by UID.
func (c *Client) GetMessage(accountID, mailbox string, uid uint32) (*domain.Message, error) {
	return c.messages.GetMessage(accountID, mailbox, uid)
}

// MarkRead marks a message as read in the given mailbox.
func (c *Client) MarkRead(accountID, mailbox string, uid uint32) error {
	return c.messages.MarkRead(accountID, mailbox, uid)
}

// MarkUnread marks a message as unread in the given mailbox.
func (c *Client) MarkUnread(accountID, mailbox string, uid uint32) error {
	return c.messages.MarkUnread(accountID, mailbox, uid)
}

// FlagMessage flags a message in the given mailbox.
func (c *Client) FlagMessage(accountID, mailbox string, uid uint32) error {
	return c.messages.FlagMessage(accountID, mailbox, uid)
}

// UnflagMessage unflags a message in the given mailbox.
func (c *Client) UnflagMessage(accountID, mailbox string, uid uint32) error {
	return c.messages.UnflagMessage(accountID, mailbox, uid)
}

// --- Events ---

// Events returns a read-only channel for receiving client events.
func (c *Client) Events() <-chan Event {
	return c.events
}

// --- Lifecycle ---

// Close disconnects all accounts and closes the event channel.
// It is safe to call multiple times; only the first call has any effect.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		c.manager.DisconnectAll()
		close(c.events)
	})
	return nil
}

// emit sends an event to the events channel without blocking.
// It is a no-op after Close has been called.
func (c *Client) emit(e Event) {
	if c.closed.Load() {
		return
	}
	select {
	case c.events <- e:
	default:
		// Channel full, drop event to avoid blocking.
		// Future: log this or use a ring buffer.
	}
}
