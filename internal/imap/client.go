package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"strings"

	"github.com/Araozu/go-mail/internal/auth"
	"github.com/Araozu/go-mail/internal/domain"
	goiap "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
)

// Client wraps a single IMAP connection for one account.
// It is NOT safe for concurrent use — the caller must serialize access.
type Client struct {
	account  *domain.Account
	provider auth.Provider
	conn     *client.Client
	host     string
}

// NewClient creates a new IMAP client for the given account.
// host should be in "imap.gmail.com:993" format.
func NewClient(account *domain.Account, provider auth.Provider, host string) *Client {
	return &Client{
		account:  account,
		provider: provider,
		host:     host,
	}
}

// Connect dials the IMAP server over TLS and authenticates.
func (c *Client) Connect() error {
	conn, err := client.DialTLS(c.host, &tls.Config{})
	if err != nil {
		return fmt.Errorf("dialing %s: %w", c.host, err)
	}
	c.conn = conn

	saslClient, err := c.provider.SASLClient(c.account.ID)
	if err != nil {
		c.conn.Logout()
		c.conn = nil
		return fmt.Errorf("creating SASL client: %w", err)
	}

	if err := c.conn.Authenticate(saslClient); err != nil {
		c.conn.Logout()
		c.conn = nil
		return fmt.Errorf("authenticating: %w", err)
	}

	return nil
}

// Disconnect logs out and closes the connection.
func (c *Client) Disconnect() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Logout()
	c.conn = nil
	return err
}

// ListMailboxes returns all mailboxes for the account.
func (c *Client) ListMailboxes() ([]*domain.Mailbox, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	ch := make(chan *goiap.MailboxInfo, 20)
	done := make(chan error, 1)
	go func() {
		done <- c.conn.List("", "*", ch)
	}()

	var mailboxes []*domain.Mailbox
	for info := range ch {
		mailboxes = append(mailboxes, &domain.Mailbox{
			Name:       info.Name,
			Delimiter:  info.Delimiter,
			Attributes: info.Attributes,
		})
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("listing mailboxes: %w", err)
	}

	return mailboxes, nil
}

// SelectMailbox selects a mailbox and returns its status.
func (c *Client) SelectMailbox(name string) (*domain.Mailbox, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	status, err := c.conn.Select(name, false)
	if err != nil {
		return nil, fmt.Errorf("selecting %s: %w", name, err)
	}

	return &domain.Mailbox{
		Name:     status.Name,
		Messages: status.Messages,
		Unseen:   status.Unseen,
	}, nil
}

// FetchEnvelopes fetches message envelopes from the currently selected mailbox.
// It returns up to `limit` most recent messages.
func (c *Client) FetchEnvelopes(limit uint32) ([]*domain.Envelope, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	mbox := c.conn.Mailbox()
	if mbox == nil {
		return nil, fmt.Errorf("no mailbox selected")
	}

	if mbox.Messages == 0 {
		return nil, nil
	}

	from := uint32(1)
	if mbox.Messages > limit {
		from = mbox.Messages - limit + 1
	}

	seqSet := new(goiap.SeqSet)
	seqSet.AddRange(from, mbox.Messages)

	items := []goiap.FetchItem{goiap.FetchUid, goiap.FetchEnvelope, goiap.FetchFlags}

	ch := make(chan *goiap.Message, 20)
	done := make(chan error, 1)
	go func() {
		done <- c.conn.Fetch(seqSet, items, ch)
	}()

	var envelopes []*domain.Envelope
	for msg := range ch {
		if msg.Envelope == nil {
			continue
		}
		envelopes = append(envelopes, convertEnvelope(msg))
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetching envelopes: %w", err)
	}

	return envelopes, nil
}

// FetchMessage fetches a full message by UID from the currently selected mailbox.
func (c *Client) FetchMessage(uid uint32) (*domain.Message, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	seqSet := new(goiap.SeqSet)
	seqSet.AddNum(uid)

	section := &goiap.BodySectionName{}
	items := []goiap.FetchItem{
		goiap.FetchUid,
		goiap.FetchEnvelope,
		goiap.FetchFlags,
		section.FetchItem(),
	}

	ch := make(chan *goiap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- c.conn.UidFetch(seqSet, items, ch)
	}()

	msg := <-ch
	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetching message %d: %w", uid, err)
	}
	if msg == nil {
		return nil, fmt.Errorf("message %d not found", uid)
	}

	result := &domain.Message{
		Envelope: *convertEnvelope(msg),
	}

	// Parse MIME body
	body := msg.GetBody(section)
	if body != nil {
		textBody, htmlBody := parseMessageBody(body)
		result.TextBody = textBody
		result.HTMLBody = htmlBody
	}

	return result, nil
}

// SetFlags replaces the flags on a message identified by UID.
func (c *Client) SetFlags(uid uint32, flags []string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	seqSet := new(goiap.SeqSet)
	seqSet.AddNum(uid)

	flagValues := make([]interface{}, len(flags))
	for i, f := range flags {
		flagValues[i] = f
	}

	item := goiap.FormatFlagsOp(goiap.SetFlags, true)
	return c.conn.UidStore(seqSet, item, flagValues, nil)
}

// AddFlags adds flags to a message identified by UID.
func (c *Client) AddFlags(uid uint32, flags []string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	seqSet := new(goiap.SeqSet)
	seqSet.AddNum(uid)

	flagValues := make([]interface{}, len(flags))
	for i, f := range flags {
		flagValues[i] = f
	}

	item := goiap.FormatFlagsOp(goiap.AddFlags, true)
	return c.conn.UidStore(seqSet, item, flagValues, nil)
}

// RemoveFlags removes flags from a message identified by UID.
func (c *Client) RemoveFlags(uid uint32, flags []string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	seqSet := new(goiap.SeqSet)
	seqSet.AddNum(uid)

	flagValues := make([]interface{}, len(flags))
	for i, f := range flags {
		flagValues[i] = f
	}

	item := goiap.FormatFlagsOp(goiap.RemoveFlags, true)
	return c.conn.UidStore(seqSet, item, flagValues, nil)
}

// convertEnvelope maps a go-imap Message to our domain Envelope.
func convertEnvelope(msg *goiap.Message) *domain.Envelope {
	env := msg.Envelope
	return &domain.Envelope{
		UID:       msg.Uid,
		MessageID: env.MessageId,
		Subject:   env.Subject,
		From:      convertAddresses(env.From),
		To:        convertAddresses(env.To),
		Cc:        convertAddresses(env.Cc),
		Date:      env.Date,
		Flags:     msg.Flags,
	}
}

// convertAddresses maps go-imap addresses to our domain addresses.
func convertAddresses(addrs []*goiap.Address) []*domain.Address {
	if len(addrs) == 0 {
		return nil
	}
	result := make([]*domain.Address, len(addrs))
	for i, a := range addrs {
		result[i] = &domain.Address{
			Name:    a.PersonalName,
			Address: a.Address(),
		}
	}
	return result
}

// parseMessageBody reads a MIME message and extracts text and HTML bodies.
func parseMessageBody(r io.Reader) (textBody, htmlBody string) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return "", ""
	}

	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			body, readErr := io.ReadAll(p.Body)
			if readErr != nil {
				continue
			}

			switch {
			case strings.HasPrefix(contentType, "text/plain"):
				textBody = string(body)
			case strings.HasPrefix(contentType, "text/html"):
				htmlBody = string(body)
			}
		}
	}

	return textBody, htmlBody
}
