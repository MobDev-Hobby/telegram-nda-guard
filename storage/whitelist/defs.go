// Package whitelist holds the per-channel whitelist model. A whitelisted user
// skips the access check in that channel until the approval expires; it must
// then be re-approved by a channel administrator.
package whitelist

import "time"

// Entry is one whitelisted user of one channel.
type Entry struct {
	UserID    int64  `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName,omitempty"`
	Username  string `json:"username,omitempty"`

	// ApprovedBy is the Telegram user ID of the administrator who added or
	// last re-approved the entry.
	ApprovedBy int64     `json:"approvedBy"`
	ApprovedAt time.Time `json:"approvedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`

	// RemindedFor is the ExpiresAt value a "expires soon" reminder was sent
	// for, so each approval period is reminded about once.
	RemindedFor time.Time `json:"remindedFor"`
	// ExpiryNotified is set once the "expired" notice has been sent; cleared
	// by re-approval.
	ExpiryNotified bool `json:"expiryNotified,omitempty"`
}

// Active reports whether the approval is still in force at now.
func (e Entry) Active(now time.Time) bool {
	return now.Before(e.ExpiresAt)
}
