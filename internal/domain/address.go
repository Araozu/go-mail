package domain

import "fmt"

// Address represents an email address with an optional display name.
type Address struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// String returns the address formatted as "Name <address>" or just "address".
// Returns "" if the receiver is nil.
func (a *Address) String() string {
	if a == nil {
		return ""
	}
	if a.Name != "" {
		return fmt.Sprintf("%s <%s>", a.Name, a.Address)
	}
	return a.Address
}
