package mailclient

// Options configures the mail client.
type Options struct {
	// ConfigDir is the directory for storing account configs and tokens.
	// Defaults to ~/.config/go-mail
	ConfigDir string

	// GmailClientID is the OAuth2 client ID for Gmail.
	GmailClientID string

	// GmailClientSecret is the OAuth2 client secret for Gmail.
	GmailClientSecret string

	// GmailRedirectURL is the OAuth2 redirect URL for Gmail.
	// Defaults to "http://localhost:8080/callback"
	GmailRedirectURL string

	// EventBufferSize is the buffer size of the events channel.
	// Defaults to 64.
	EventBufferSize int
}

// Option is a function that configures Options.
type Option func(*Options)

// WithConfigDir sets the configuration directory.
func WithConfigDir(dir string) Option {
	return func(o *Options) {
		o.ConfigDir = dir
	}
}

// WithGmailCredentials sets the Gmail OAuth2 credentials.
func WithGmailCredentials(clientID, clientSecret string) Option {
	return func(o *Options) {
		o.GmailClientID = clientID
		o.GmailClientSecret = clientSecret
	}
}

// WithGmailRedirectURL sets the Gmail OAuth2 redirect URL.
func WithGmailRedirectURL(url string) Option {
	return func(o *Options) {
		o.GmailRedirectURL = url
	}
}

// WithEventBufferSize sets the event channel buffer size.
// Negative values are clamped to 0.
func WithEventBufferSize(size int) Option {
	return func(o *Options) {
		if size < 0 {
			size = 0
		}
		o.EventBufferSize = size
	}
}

func defaultOptions() *Options {
	return &Options{
		GmailRedirectURL: "http://localhost:8080/callback",
		EventBufferSize:  64,
	}
}
