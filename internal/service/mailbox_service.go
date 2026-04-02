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
// Returns an error if manager is nil.
func NewMailboxService(manager *imapPkg.Manager) (*MailboxService, error) {
	if manager == nil {
		return nil, fmt.Errorf("nil IMAP manager")
	}
	return &MailboxService{manager: manager}, nil
}

// ListMailboxes returns all mailboxes for the given account.
func (s *MailboxService) ListMailboxes(accountID string) ([]*domain.Mailbox, error) {
	var mailboxes []*domain.Mailbox
	err := s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		var listErr error
		mailboxes, listErr = c.ListMailboxes()
		return listErr
	})
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}
	return mailboxes, nil
}

// SelectMailbox selects a mailbox and returns its status.
func (s *MailboxService) SelectMailbox(accountID, name string) (*domain.Mailbox, error) {
	var mbox *domain.Mailbox
	err := s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		var selectErr error
		mbox, selectErr = c.SelectMailbox(name)
		return selectErr
	})
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}
	return mbox, nil
}
