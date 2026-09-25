// Package knownchats remembers chats where the bot was made an administrator,
// so the Mini App can offer the ones that are not protected yet.
package knownchats

import "time"

// Chat is a chat the bot administers.
type Chat struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
	// AddedBy is who made the bot an administrator.
	AddedBy            int64     `json:"addedBy,omitempty"`
	AddedAt            time.Time `json:"addedAt"`
	CanRestrictMembers bool      `json:"canRestrictMembers"`
	CanInviteUsers     bool      `json:"canInviteUsers"`
}
