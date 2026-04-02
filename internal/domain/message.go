package domain

// Message is a full email message including body content.
type Message struct {
	Envelope
	TextBody string `json:"text_body"`
	HTMLBody string `json:"html_body"`
}
