package auth

import (
	"github.com/emersion/go-sasl"
)

// Provider is the interface for authentication providers.
// Each provider (Gmail, Outlook, etc.) implements this to handle
// its own auth flow and produce a SASL client for IMAP login.
type Provider interface {
	// Name returns the provider identifier (e.g. "gmail").
	Name() string

	// StartFlow begins the authorization flow and returns a URL
	// the user must visit to authorize the application.
	StartFlow() (authURL string, err error)

	// CompleteFlowWithAccount exchanges the authorization code for tokens
	// and persists them under the given account ID.
	CompleteFlowWithAccount(code, accountID string) error

	// SASLClient returns a SASL client for IMAP authentication
	// using stored tokens for the given account.
	SASLClient(accountID string) (sasl.Client, error)
}
