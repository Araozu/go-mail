# go-mail Architecture

IMAP mail client written in Go. Handles multiple inboxes with OAuth2 support for Gmail.

## Design Principles

- **No rendering assumptions**: the core makes zero assumptions about how it will be presented
- **Layered separation**: each layer has a single responsibility and clear dependency direction
- **Interface-driven**: layers communicate through interfaces, enabling testing and swapping implementations
- **Multi-account**: designed from the start to manage N inboxes concurrently

## Consumers (planned, in order)

1. **CLI** — dumb terminal client for testing IMAP operations
2. **Frontend UI** — web-based interface on top of the Go core
3. **Desktop app** — via Wails

Only the Go side is covered in this document.

## Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/emersion/go-imap` | v1 (stable) | IMAP4rev1 client |
| `github.com/emersion/go-sasl` | latest | SASL auth (OAUTHBEARER for Gmail) |
| `github.com/emersion/go-message` | latest | MIME message parsing |
| `golang.org/x/oauth2` | latest | OAuth2 token flow (Google provider) |

## Project Structure

```
go-mail/
├── cmd/
│   └── cli/                        # CLI entrypoint
│       └── main.go
│
├── internal/
│   ├── domain/                     # Layer 1: Pure domain models
│   │   ├── account.go              # Account config (email, provider, OAuth creds)
│   │   ├── mailbox.go              # Mailbox/folder model
│   │   ├── message.go              # Full message model
│   │   └── envelope.go             # Lightweight headers for listing
│   │
│   ├── auth/                       # Layer 2: Authentication
│   │   ├── oauth2.go               # OAuth2 flow (Google provider)
│   │   ├── token_store.go          # TokenStore interface + JSON file impl
│   │   └── provider.go             # AuthProvider interface
│   │
│   ├── imap/                       # Layer 3: IMAP operations
│   │   ├── client.go               # Wrapper around go-imap client
│   │   └── manager.go              # Multi-account connection manager
│   │
│   ├── service/                    # Layer 4: Business logic
│   │   ├── account_service.go      # Add/remove/list accounts
│   │   ├── mailbox_service.go      # List folders, message counts
│   │   └── message_service.go      # Fetch, search, mark read/unread, move
│   │
│   └── store/                      # Layer 5: Local persistence
│       └── config.go               # JSON file-based account config + token storage
│
└── pkg/
    └── mailclient/                 # Layer 6: Public API surface
        ├── client.go               # Main Client struct — what frontends use
        ├── options.go              # Configuration/initialization options
        └── events.go               # Event types + Go channel-based notification
```

## Layer Details

### Layer 1 — `internal/domain` (Pure Models)

Zero dependencies. Just data structures.

```go
// account.go
type Account struct {
    ID       string
    Email    string
    Provider string   // "gmail", "outlook", etc.
    AuthType string   // "oauth2", "password"
}

// mailbox.go
type Mailbox struct {
    Name       string
    Delimiter  string
    Messages   uint32
    Unseen     uint32
    Attributes []string
}

// envelope.go
type Envelope struct {
    MessageID string
    Subject   string
    From      []*Address
    To        []*Address
    Date      time.Time
    Flags     []string
}

// message.go
type Message struct {
    Envelope
    Body     string   // plain text body
    HTMLBody string   // HTML body
    // attachments TBD
}

type Address struct {
    Name    string
    Address string
}
```

### Layer 2 — `internal/auth` (Authentication)

Handles OAuth2 for Gmail. Interface-based so we can add other providers.

```go
// provider.go
type AuthProvider interface {
    // Authenticate returns a *sasl.Client ready for IMAP auth
    Authenticate(account *domain.Account) (sasl.Client, error)
    // StartFlow begins the OAuth2 authorization (opens browser, etc.)
    StartFlow() (authURL string, err error)
    // CompleteFlow exchanges the auth code for tokens
    CompleteFlow(code string) (*oauth2.Token, error)
}

// token_store.go
type TokenStore interface {
    Save(accountID string, token *oauth2.Token) error
    Load(accountID string) (*oauth2.Token, error)
    Delete(accountID string) error
}
```

OAuth2 flow for Gmail:
1. Generate auth URL with scope `https://mail.google.com/`
2. User opens URL in browser, authorizes, gets code
3. Exchange code for token pair (access + refresh)
4. Store tokens as JSON on disk
5. On IMAP connect: use `sasl.NewOAuthBearerClient` with the access token
6. On token expiry: auto-refresh via `oauth2.TokenSource`

### Layer 3 — `internal/imap` (IMAP Client)

Wraps `go-imap` v1 with connection lifecycle management.

```go
// client.go
type Client struct {
    account  *domain.Account
    imapConn *imapclient.Client
    auth     auth.AuthProvider
}

func (c *Client) Connect() error
func (c *Client) Disconnect() error
func (c *Client) ListMailboxes() ([]domain.Mailbox, error)
func (c *Client) SelectMailbox(name string) (*domain.Mailbox, error)
func (c *Client) FetchEnvelopes(mailbox string, limit uint32) ([]domain.Envelope, error)
func (c *Client) FetchMessage(mailbox string, uid uint32) (*domain.Message, error)
func (c *Client) Search(mailbox string, criteria SearchCriteria) ([]domain.Envelope, error)
func (c *Client) SetFlags(uid uint32, flags []string) error

// manager.go
type Manager struct {
    clients map[string]*Client  // keyed by account ID
}

func (m *Manager) AddAccount(account *domain.Account, provider auth.AuthProvider) error
func (m *Manager) RemoveAccount(accountID string) error
func (m *Manager) GetClient(accountID string) (*Client, error)
```

### Layer 4 — `internal/service` (Business Logic)

Orchestrates auth, IMAP, and storage. The "brain."

```go
// account_service.go
type AccountService struct {
    store         store.ConfigStore
    authProviders map[string]auth.AuthProvider
    manager       *imap.Manager
}

func (s *AccountService) AddGmailAccount() error       // full OAuth flow
func (s *AccountService) RemoveAccount(id string) error
func (s *AccountService) ListAccounts() ([]domain.Account, error)

// mailbox_service.go
type MailboxService struct {
    manager *imap.Manager
}

func (s *MailboxService) ListMailboxes(accountID string) ([]domain.Mailbox, error)

// message_service.go
type MessageService struct {
    manager *imap.Manager
}

func (s *MessageService) ListMessages(accountID, mailbox string, limit uint32) ([]domain.Envelope, error)
func (s *MessageService) GetMessage(accountID, mailbox string, uid uint32) (*domain.Message, error)
func (s *MessageService) MarkRead(accountID string, uid uint32) error
func (s *MessageService) MarkUnread(accountID string, uid uint32) error
func (s *MessageService) MoveMessage(accountID string, uid uint32, destMailbox string) error
```

### Layer 5 — `internal/store` (Local Persistence)

JSON files on disk. Simple and debuggable.

```go
// config.go
type ConfigStore interface {
    SaveAccount(account *domain.Account) error
    LoadAccounts() ([]domain.Account, error)
    DeleteAccount(id string) error
}

// Stored at ~/.config/go-mail/accounts.json
// Tokens at ~/.config/go-mail/tokens/<account-id>.json
```

### Layer 6 — `pkg/mailclient` (Public API)

The only exported package. This is what CLI, UI, and Wails consume.

```go
// client.go
type Client struct {
    accounts  *service.AccountService
    mailboxes *service.MailboxService
    messages  *service.MessageService
    events    chan Event
}

func New(opts ...Option) (*Client, error)
func (c *Client) AddGmailAccount() error
func (c *Client) ListAccounts() ([]domain.Account, error)
func (c *Client) ListMailboxes(accountID string) ([]domain.Mailbox, error)
func (c *Client) ListMessages(accountID, mailbox string, limit uint32) ([]domain.Envelope, error)
func (c *Client) GetMessage(accountID, mailbox string, uid uint32) (*domain.Message, error)
func (c *Client) Events() <-chan Event
func (c *Client) Close() error

// events.go
type EventType string

const (
    EventNewMessage  EventType = "new_message"
    EventError       EventType = "error"
    EventSyncStarted EventType = "sync_started"
    EventSyncDone    EventType = "sync_done"
)

type Event struct {
    Type      EventType
    AccountID string
    Data      any
}
```

## Dependency Flow

```
cmd/cli ──→ pkg/mailclient ──→ internal/service ──→ internal/imap ──→ go-imap v1
                                                 ──→ internal/auth ──→ golang.org/x/oauth2
                                                 ──→ internal/store
                             ──→ internal/domain (used by all layers)
```

No layer may import a layer above it. `domain` is imported by everyone but imports nothing.

## CLI Commands (First Consumer)

| Command | Description |
|---------|-------------|
| `accounts add` | Start OAuth flow, add a Gmail account |
| `accounts list` | List all configured accounts |
| `accounts remove <id>` | Remove an account |
| `mailboxes <account>` | List folders for an account |
| `messages <account> <mailbox>` | List recent messages |
| `read <account> <uid>` | Read a full message |

## Future Phases

- **IDLE support**: dedicated connection per watched mailbox for real-time new mail notifications
- **Connection pooling**: multiple IMAP connections per account for concurrent operations
- **Local cache**: SQLite-backed message cache for offline access
- **Frontend UI**: web interface consuming `pkg/mailclient`
- **Wails desktop app**: native wrapper around the web UI
