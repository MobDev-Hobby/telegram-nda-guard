// Package whitelist holds the per-channel whitelist model. A whitelisted user
// skips the access check in that channel. When the approval expires
// (ExpiresAt) the entry keeps protecting the user and administrators are
// reminded to review it. Access ends only when a temporary entry reaches
// DeleteAt or an administrator removes the entry.
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

	// Note is the approver's free-text reason.
	Note string `json:"note,omitempty"`
	// DeleteAt, when set, is when the entry is removed for good (a temporary
	// approval). Until then the regular review every ExpiresAt still applies.
	DeleteAt *time.Time `json:"deleteAt,omitempty"`

	// RemindedFor is the ExpiresAt value a "expires soon" reminder was sent
	// for, so each approval period is reminded about once.
	RemindedFor time.Time `json:"remindedFor"`
	// ExpiredRemindedAt is when the last "expired, please review" reminder
	// went out; expired entries are reminded about daily.
	ExpiredRemindedAt time.Time `json:"expiredRemindedAt"`
}

// Expired reports whether the approval is past its term at now. An expired
// entry still protects the user; it only asks for a review.
func (e Entry) Expired(now time.Time) bool {
	return !now.Before(e.ExpiresAt)
}

// Deleted reports whether a temporary entry has reached its end at now and no
// longer applies.
func (e Entry) Deleted(now time.Time) bool {
	return e.DeleteAt != nil && !now.Before(*e.DeleteAt)
}

// ReviewDue reports whether the entry is removed before its next review, i.e.
// the review reminder is moot.
func (e Entry) EndsBeforeReview() bool {
	return e.DeleteAt != nil && !e.DeleteAt.After(e.ExpiresAt)
}
