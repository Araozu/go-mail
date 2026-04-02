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
	var envelopes []*domain.Envelope
	err := s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		var fetchErr error
		envelopes, fetchErr = c.FetchEnvelopes(limit)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	return envelopes, nil
}

// GetMessage fetches a full message by UID. The mailbox must be provided
// so it can be selected before fetching.
func (s *MessageService) GetMessage(accountID, mailbox string, uid uint32) (*domain.Message, error) {
	var msg *domain.Message
	err := s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		var fetchErr error
		msg, fetchErr = c.FetchMessage(uid)
		return fetchErr
	})
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// MarkRead adds the \Seen flag to a message in the given mailbox.
func (s *MessageService) MarkRead(accountID, mailbox string, uid uint32) error {
	return s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		return c.AddFlags(uid, []string{goimap.SeenFlag})
	})
}

// MarkUnread removes the \Seen flag from a message in the given mailbox.
func (s *MessageService) MarkUnread(accountID, mailbox string, uid uint32) error {
	return s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		return c.RemoveFlags(uid, []string{goimap.SeenFlag})
	})
}

// FlagMessage adds the \Flagged flag to a message in the given mailbox.
func (s *MessageService) FlagMessage(accountID, mailbox string, uid uint32) error {
	return s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		return c.AddFlags(uid, []string{goimap.FlaggedFlag})
	})
}

// UnflagMessage removes the \Flagged flag from a message in the given mailbox.
func (s *MessageService) UnflagMessage(accountID, mailbox string, uid uint32) error {
	return s.manager.DoWithClient(accountID, func(c *imapPkg.Client) error {
		if _, err := c.SelectMailbox(mailbox); err != nil {
			return fmt.Errorf("selecting mailbox: %w", err)
		}
		return c.RemoveFlags(uid, []string{goimap.FlaggedFlag})
	})
}
