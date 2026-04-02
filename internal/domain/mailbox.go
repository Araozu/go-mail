package domain

// Mailbox represents an IMAP mailbox (folder).
type Mailbox struct {
	Name       string   `json:"name"`
	Delimiter  string   `json:"delimiter"`
	Messages   uint32   `json:"messages"`
	Unseen     uint32   `json:"unseen"`
	Attributes []string `json:"attributes"`
}
