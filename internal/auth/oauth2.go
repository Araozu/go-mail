package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const gmailIMAPScope = "https://mail.google.com/"

// GmailProvider implements Provider for Google accounts using OAuth2.
type GmailProvider struct {
	config     *oauth2.Config
	tokenStore TokenStore

	// pendingState holds the CSRF state for an in-progress OAuth flow.
	pendingState string
}

// NewGmailProvider creates a new Gmail auth provider.
func NewGmailProvider(clientID, clientSecret, redirectURL string, tokenStore TokenStore) *GmailProvider {
	return &GmailProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{gmailIMAPScope},
			Endpoint:     google.Endpoint,
		},
		tokenStore: tokenStore,
	}
}

// Name returns the provider identifier.
func (g *GmailProvider) Name() string {
	return "gmail"
}

// StartFlow generates an authorization URL the user must visit.
func (g *GmailProvider) StartFlow() (string, error) {
	state, err := randomState()
	if err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}
	g.pendingState = state

	url := g.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	return url, nil
}

// CompleteFlowWithAccount exchanges the auth code and saves the token
// under the given account ID.
func (g *GmailProvider) CompleteFlowWithAccount(code, accountID string) error {
	token, err := g.config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchanging code: %w", err)
	}

	if err := g.tokenStore.Save(accountID, token); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	g.pendingState = ""
	return nil
}

// SASLClient returns an OAUTHBEARER SASL client for IMAP auth.
// It loads the stored token for the account, refreshes it if needed,
// and returns a ready-to-use SASL client.
func (g *GmailProvider) SASLClient(accountID string) (sasl.Client, error) {
	token, err := g.tokenStore.Load(accountID)
	if err != nil {
		return nil, fmt.Errorf("loading token: %w", err)
	}

	// Create a token source that auto-refreshes
	ts := g.config.TokenSource(context.Background(), token)
	freshToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	// Persist the refreshed token if it changed
	if freshToken.AccessToken != token.AccessToken {
		if err := g.tokenStore.Save(accountID, freshToken); err != nil {
			return nil, fmt.Errorf("saving refreshed token: %w", err)
		}
	}

	return sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
		Username: accountID,
		Token:    freshToken.AccessToken,
	}), nil
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
