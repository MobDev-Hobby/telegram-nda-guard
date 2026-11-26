package processors

import (
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// CleanOptions controls how a cleaner removes users from a channel.
type CleanOptions struct {
	// KeepBanned leaves removed users banned instead of just kicking them.
	KeepBanned bool `json:"keepBanned"`
	// CleanMessages revokes the removed member's access to chat history.
	// Telegram always applies this in supergroups and channels (banChatMember).
	// It does not delete the member's own messages for everyone.
	CleanMessages bool `json:"cleanMessages"`
	// CleanUnknown also removes users whose access could not be checked.
	CleanUnknown bool `json:"cleanUnknown"`
}

// ScanSchedule says when a channel's members are checked: a time of day, the
// days it applies to, and the zone the time is read in.
//
// It is stored per channel, like CleanOptions: absent means "use the service
// default". Records written before it existed simply have no schedule.
type ScanSchedule struct {
	// At is the time of day in 24-hour "HH:MM" form.
	At string `json:"at"`
	// Weekdays limits the schedule to those days. Empty means every day.
	Weekdays []time.Weekday `json:"weekdays,omitempty"`
	// TZ is an IANA name such as "Europe/Moscow". Empty means the service
	// default, so a channel inherits it unless it says otherwise.
	TZ string `json:"tz,omitempty"`
}

// NotificationOptions controls bot messages independently of member checks.
// Modes: all (full summaries), violations_summary (full summaries only with
// issues), violations_only (only failing/unchecked members and partial scans).
type NotificationOptions struct {
	Mode          string `json:"mode"`
	IntervalHours int    `json:"intervalHours"`
	// At releases the summary at a time of day ("HH:MM") instead of every
	// IntervalHours. The first scan at or after that time each day reports;
	// later scans the same day stay quiet. It is read in the channel's scan
	// schedule zone. Empty keeps the IntervalHours behaviour.
	At string `json:"at,omitempty"`
}

type AccessReport struct {
	ViolationsOnly bool
	Channel        guard.ChannelInfo
	ReportChannels []int64
	AllowedUsers   []guard.User
	DeniedUsers    []guard.User
	UnknownUsers   []guard.User
	// CleanOptions, when set, overrides the cleaner's process-wide defaults
	// for this channel.
	CleanOptions *CleanOptions
	// Stats tells how complete the member list was. Zero value = unknown.
	Stats guard.ScanStats
}

// KickResult is the outcome of removing one user.
type KickResult struct {
	UserID int64  `json:"userId"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// CheckSummary is the outcome of the latest member check of a channel, kept to
// show the channel's health.
type CheckSummary struct {
	At          time.Time `json:"at"`
	Good        int       `json:"good"`
	Bad         int       `json:"bad"`
	Unknown     int       `json:"unknown"`
	Whitelisted int       `json:"whitelisted"`
	Partial     bool      `json:"partial,omitempty"`
}
