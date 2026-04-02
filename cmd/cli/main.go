package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Araozu/go-mail/pkg/mailclient"
)

func main() {
	// Gmail credentials from environment variables
	clientID := os.Getenv("GMAIL_CLIENT_ID")
	clientSecret := os.Getenv("GMAIL_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		fmt.Println("Set GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET environment variables")
		os.Exit(1)
	}

	client, err := mailclient.New(
		mailclient.WithGmailCredentials(clientID, clientSecret),
	)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Try connecting existing accounts
	ctx := context.Background()
	if err := client.ConnectAll(ctx); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("go-mail CLI")
	fmt.Println("Commands: accounts list | accounts add | mailboxes <id> | messages <id> <mailbox> [limit] | read <id> <mailbox> <uid> | quit")
	fmt.Println()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		args := strings.Fields(line)
		cmd := args[0]

		switch cmd {
		case "quit", "exit", "q":
			fmt.Println("Bye!")
			return

		case "accounts":
			handleAccounts(client, scanner, args[1:])

		case "mailboxes":
			handleMailboxes(client, args[1:])

		case "messages":
			handleMessages(client, args[1:])

		case "read":
			handleRead(client, args[1:])

		case "help":
			fmt.Println("Commands:")
			fmt.Println("  accounts list          - List configured accounts")
			fmt.Println("  accounts add           - Add a Gmail account via OAuth")
			fmt.Println("  accounts remove <id>   - Remove an account")
			fmt.Println("  mailboxes <account-id> - List mailboxes for an account")
			fmt.Println("  messages <account-id> <mailbox> [limit]")
			fmt.Println("                         - List recent messages")
			fmt.Println("  read <account-id> <mailbox> <uid>")
			fmt.Println("                         - Read a full message")
			fmt.Println("  quit                   - Exit")

		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func handleAccounts(client *mailclient.Client, scanner *bufio.Scanner, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: accounts [list|add|remove <id>]")
		return
	}

	switch args[0] {
	case "list":
		accounts, err := client.ListAccounts()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		if len(accounts) == 0 {
			fmt.Println("No accounts configured. Use 'accounts add' to add one.")
			return
		}
		for _, a := range accounts {
			fmt.Printf("  [%s] %s (%s, %s)\n", a.ID, a.Email, a.Provider, a.AuthType)
		}

	case "add":
		addAccount(client, scanner)

	case "remove":
		if len(args) < 2 {
			fmt.Println("Usage: accounts remove <id>")
			return
		}
		if err := client.RemoveAccount(args[1]); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("Account removed.")

	default:
		fmt.Printf("Unknown accounts subcommand: %s\n", args[0])
	}
}

func addAccount(client *mailclient.Client, scanner *bufio.Scanner) {
	ctx := context.Background()

	// Ask for email first (we need it for account creation)
	fmt.Print("Enter your Gmail address: ")
	if !scanner.Scan() {
		return
	}
	email := strings.TrimSpace(scanner.Text())

	authURL, err := client.StartAddAccount("gmail")
	if err != nil {
		fmt.Printf("Error starting auth: %v\n", err)
		return
	}

	// Start a temporary HTTP server to catch the OAuth callback
	type callbackResult struct {
		code  string
		state string
	}
	resultCh := make(chan callbackResult, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code in callback", http.StatusBadRequest)
			errCh <- fmt.Errorf("no code in OAuth callback")
			return
		}
		state := r.URL.Query().Get("state")

		fmt.Fprintf(w, "<html><body><h2>Authorization successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		resultCh <- callbackResult{code: code, state: state}
	})

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Printf("Error starting callback server: %v\n", err)
		fmt.Println("Port 8080 may be in use. Free it and try again.")
		return
	}

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Println()
	fmt.Println("Open this URL in your browser:")
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	// Wait for the callback or an error
	var result callbackResult
	select {
	case result = <-resultCh:
		// Got the code and state!
	case err := <-errCh:
		srv.Shutdown(ctx)
		fmt.Printf("Error during auth: %v\n", err)
		return
	}

	// Shut down the temporary server
	srv.Shutdown(ctx)

	account, err := client.CompleteAddAccount(ctx, "gmail", email, result.code, result.state)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Account added: [%s] %s\n", account.ID, account.Email)

	// Try connecting immediately
	if err := client.ConnectAll(ctx); err != nil {
		fmt.Printf("Warning connecting: %v\n", err)
	}
}

func handleMailboxes(client *mailclient.Client, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: mailboxes <account-id>")
		return
	}

	mailboxes, err := client.ListMailboxes(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, m := range mailboxes {
		attrs := ""
		if len(m.Attributes) > 0 {
			attrs = " [" + strings.Join(m.Attributes, ", ") + "]"
		}
		fmt.Printf("  %s (msgs: %d, unseen: %d)%s\n", m.Name, m.Messages, m.Unseen, attrs)
	}
}

func handleMessages(client *mailclient.Client, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: messages <account-id> <mailbox> [limit]")
		return
	}

	accountID := args[0]
	mailbox := args[1]
	limit := uint32(20)

	if len(args) >= 3 {
		n, err := strconv.ParseUint(args[2], 10, 32)
		if err != nil {
			fmt.Printf("Invalid limit: %s\n", args[2])
			return
		}
		limit = uint32(n)
	}

	envelopes, err := client.ListMessages(accountID, mailbox, limit)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if len(envelopes) == 0 {
		fmt.Println("No messages.")
		return
	}

	for _, env := range envelopes {
		from := "?"
		if len(env.From) > 0 {
			from = env.From[0].String()
		}

		flags := ""
		if len(env.Flags) > 0 {
			flags = " " + strings.Join(env.Flags, " ")
		}

		fmt.Printf("  UID:%d  %s  %-30s  %s%s\n",
			env.UID,
			env.Date.Format("Jan 02 15:04"),
			truncate(from, 30),
			truncate(env.Subject, 50),
			flags,
		)
	}
}

func handleRead(client *mailclient.Client, args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: read <account-id> <mailbox> <uid>")
		return
	}

	accountID := args[0]
	mailbox := args[1]
	uid, err := strconv.ParseUint(args[2], 10, 32)
	if err != nil {
		fmt.Printf("Invalid UID: %s\n", args[2])
		return
	}

	msg, err := client.GetMessage(accountID, mailbox, uint32(uid))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Subject: %s\n", msg.Subject)
	fmt.Printf("Date:    %s\n", msg.Date.Format("Mon, 02 Jan 2006 15:04:05 MST"))

	if len(msg.From) > 0 {
		from := make([]string, len(msg.From))
		for i, a := range msg.From {
			from[i] = a.String()
		}
		fmt.Printf("From:    %s\n", strings.Join(from, ", "))
	}

	if len(msg.To) > 0 {
		to := make([]string, len(msg.To))
		for i, a := range msg.To {
			to[i] = a.String()
		}
		fmt.Printf("To:      %s\n", strings.Join(to, ", "))
	}

	fmt.Printf("Flags:   %s\n", strings.Join(msg.Flags, " "))
	fmt.Println(strings.Repeat("-", 60))

	if msg.TextBody != "" {
		fmt.Println(msg.TextBody)
	} else if msg.HTMLBody != "" {
		fmt.Println("[HTML content — text body not available]")
		fmt.Println(msg.HTMLBody)
	} else {
		fmt.Println("[No body content]")
	}
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if max <= 3 {
		return strings.Repeat(".", max)
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
