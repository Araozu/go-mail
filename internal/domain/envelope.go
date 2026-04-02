package domain

import "time"

// Envelope holds lightweight message headers for listing without fetching the body.
type Envelope struct {
	UID       uint32     `json:"uid"`
	MessageID string     `json:"message_id"`
	Subject   string     `json:"subject"`
	From      []*Address `json:"from"`
	To        []*Address `json:"to"`
	Cc        []*Address `json:"cc"`
	Date      time.Time  `json:"date"`
	Flags     []string   `json:"flags"`
}
