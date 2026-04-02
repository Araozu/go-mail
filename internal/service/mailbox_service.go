package service

import (
	"fmt"

	"github.com/Araozu/go-mail/internal/domain"
	imapPkg "github.com/Araozu/go-mail/internal/imap"
)

// MailboxService handles mailbox listing and selection.
type MailboxService struct {
	manager *imapPkg.Manager
}

// NewMailboxService creates a new mailbox service.
func NewMailboxService(manager *imapPkg.Manager) *MailboxService {
	return &MailboxService{manager: manager}
}

// ListMailboxes returns all mailboxes for the given account.
func (s *MailboxService) ListMailboxes(accountID string) ([]*domain.Mailbox, error) {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}
	return c.ListMailboxes()
}

// SelectMailbox selects a mailbox and returns its status.
func (s *MailboxService) SelectMailbox(accountID, name string) (*domain.Mailbox, error) {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}
	return c.SelectMailbox(name)
}
