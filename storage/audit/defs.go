// Package audit holds the per-channel action log shown in the Mini App.
package audit

import "time"

// Actions recorded in the log.
const (
	ActionChannelAdded     = "channel.added"
	ActionChannelJoined    = "channel.joined"
	ActionSettingsChanged  = "settings.changed"
	ActionScanCompleted    = "scan.completed"
	ActionCleanCompleted   = "clean.completed"
	ActionUsersKicked      = "users.kicked"
	ActionUserRechecked    = "user.rechecked"
	ActionWhitelistAdded   = "whitelist.added"
	ActionWhitelistRenewed = "whitelist.renewed"
	ActionWhitelistRemoved = "whitelist.removed"
	ActionWhitelistExpired = "whitelist.expired"
	ActionJoinRequested    = "join.requested"
	ActionJoinApproved     = "join.approved"
	ActionJoinDeclined     = "join.declined"
	ActionJoinRechecked    = "join.rechecked"
	ActionChatConnected    = "channel.connected"
)

// User is a person an event is about.
type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name,omitempty"`
	Username string `json:"username,omitempty"`
}

// Event is one entry of a channel's action log. It is structured so the Mini
// App can render it in the viewer's language.
type Event struct {
	At time.Time `json:"at"`
	// ActorID is who did it; 0 means the bot itself (schedules, reminders).
	ActorID   int64          `json:"actorId,omitempty"`
	ActorName string         `json:"actorName,omitempty"`
	Action    string         `json:"action"`
	Users     []User         `json:"users,omitempty"`
	Note      string         `json:"note,omitempty"`
	Counts    map[string]int `json:"counts,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}
