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

// NotificationOptions controls bot messages independently of member checks.
// Modes: all (full summaries), violations_summary (full summaries only with
// issues), violations_only (only failing/unchecked members and partial scans).
type NotificationOptions struct {
	Mode          string `json:"mode"`
	IntervalHours int    `json:"intervalHours"`
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
