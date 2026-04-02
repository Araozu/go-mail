package service

import (
	"fmt"

	goimap "github.com/emersion/go-imap"

	"github.com/Araozu/go-mail/internal/domain"
	imapPkg "github.com/Araozu/go-mail/internal/imap"
)

// MessageService handles message operations: list, read, flag changes.
type MessageService struct {
	manager *imapPkg.Manager
}

// NewMessageService creates a new message service.
func NewMessageService(manager *imapPkg.Manager) *MessageService {
	return &MessageService{manager: manager}
}

// ListMessages fetches the most recent envelopes from a mailbox.
// It selects the mailbox first, then fetches up to `limit` messages.
func (s *MessageService) ListMessages(accountID, mailbox string, limit uint32) ([]*domain.Envelope, error) {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}

	if _, err := c.SelectMailbox(mailbox); err != nil {
		return nil, fmt.Errorf("selecting mailbox: %w", err)
	}

	return c.FetchEnvelopes(limit)
}

// GetMessage fetches a full message by UID. The mailbox must be provided
// so it can be selected before fetching.
func (s *MessageService) GetMessage(accountID, mailbox string, uid uint32) (*domain.Message, error) {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return nil, fmt.Errorf("getting client: %w", err)
	}

	if _, err := c.SelectMailbox(mailbox); err != nil {
		return nil, fmt.Errorf("selecting mailbox: %w", err)
	}

	return c.FetchMessage(uid)
}

// MarkRead adds the \Seen flag to a message.
func (s *MessageService) MarkRead(accountID string, uid uint32) error {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	return c.AddFlags(uid, []string{goimap.SeenFlag})
}

// MarkUnread removes the \Seen flag from a message.
func (s *MessageService) MarkUnread(accountID string, uid uint32) error {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	return c.RemoveFlags(uid, []string{goimap.SeenFlag})
}

// FlagMessage adds the \Flagged flag to a message.
func (s *MessageService) FlagMessage(accountID string, uid uint32) error {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	return c.AddFlags(uid, []string{goimap.FlaggedFlag})
}

// UnflagMessage removes the \Flagged flag from a message.
func (s *MessageService) UnflagMessage(accountID string, uid uint32) error {
	c, err := s.manager.GetClient(accountID)
	if err != nil {
		return fmt.Errorf("getting client: %w", err)
	}

	return c.RemoveFlags(uid, []string{goimap.FlaggedFlag})
}
