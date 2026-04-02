package mailclient

// EventType identifies the kind of event.
type EventType string

const (
	// EventNewMessage is emitted when a new message is detected.
	EventNewMessage EventType = "new_message"

	// EventError is emitted when an error occurs in the background.
	EventError EventType = "error"

	// EventConnected is emitted when an account successfully connects.
	EventConnected EventType = "connected"

	// EventDisconnected is emitted when an account disconnects.
	EventDisconnected EventType = "disconnected"
)

// Event represents something that happened in the mail client.
type Event struct {
	Type      EventType   `json:"type"`
	AccountID string      `json:"account_id"`
	Data      interface{} `json:"data,omitempty"`
}
