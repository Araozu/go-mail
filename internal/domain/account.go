package domain

// Account represents a configured email account.
type Account struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Provider string `json:"provider"`  // "gmail", "outlook", etc.
	AuthType string `json:"auth_type"` // "oauth2", "password"
}
