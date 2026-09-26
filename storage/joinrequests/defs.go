// Package joinrequests keeps pending requests to join protected chats that
// approve new members. The Bot API can't list pending requests, so the bot
// records them as they arrive.
package joinrequests

import "time"

// Request is one pending request.
type Request struct {
	UserID      int64     `json:"userId"`
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName,omitempty"`
	Username    string    `json:"username,omitempty"`
	Bio         string    `json:"bio,omitempty"`
	RequestedAt time.Time `json:"requestedAt"`
	// Check is the access checker's latest verdict: good, bad or unknown.
	Check     string    `json:"check"`
	CheckedAt time.Time `json:"checkedAt"`
}
