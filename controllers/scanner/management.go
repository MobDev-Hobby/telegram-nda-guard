package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// ChannelView is a transport-neutral, read-only snapshot of a protected
// channel's state. It is a value copy (slices duplicated) so callers cannot
// mutate the controller's in-memory state through it.
//
// The in-memory fields BotOnChannel / BotCanInvite / BotCanClean reflect the
// bot's actual rights in the channel as resolved from the Telegram API; they
// are not persisted and may change between calls.
type ChannelView struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	ChatType     string  `json:"chatType"`
	CommandChats []int64 `json:"commandChats"`
	AutoScan     bool    `json:"autoScan"`
	AutoClean    bool    `json:"autoClean"`
	AllowClean   bool    `json:"allowClean"`
	BotOnChannel bool    `json:"botOnChannel"`
	BotCanInvite bool    `json:"botCanInvite"`
	BotCanClean  bool    `json:"botCanClean"`
}

// CanScan reports whether the bot can scan this channel (is a member).
func (c ChannelView) CanScan() bool { return c.BotOnChannel }

// CanClean reports whether the bot can clean (kick) users in this channel.
func (c ChannelView) CanClean() bool { return c.BotOnChannel && c.BotCanClean }

// UsersView is the classified member list for a channel: Good (access granted),
// Unknown (check failed), Bad (access denied). It mirrors what /scan computes
// but without invoking the report or cleaner processors.
type UsersView struct {
	ChannelID int64        `json:"channelId"`
	Title     string       `json:"title"`
	Good      []guard.User `json:"good"`
	Unknown   []guard.User `json:"unknown"`
	Bad       []guard.User `json:"bad"`
}

// StatusView aggregates bot/userbot identity and the full channel list, for a
// dashboard landing page.
type StatusView struct {
	BotUsername     string        `json:"botUsername"`
	BotUserID       int64         `json:"botUserId"`
	UserBotUsername string        `json:"userBotUsername"`
	UserBotUserID   int64         `json:"userBotUserId"`
	AdminChatID     int64         `json:"adminChatId"`
	Channels        []ChannelView `json:"channels"`
}

// ManagementService is the transport-neutral management surface for the
// controller. The bundled *Domain implements it; consumers (HTTP API, CLI,
// tests) depend on this interface rather than the concrete type, so the
// transport layer can be swapped without touching the domain.
//
// All methods return value copies and must be safe to call concurrently with
// the Telegram command handlers and the scan/ticker loops.
type ManagementService interface {
	// ListChannels returns the protected channels controlled from
	// commandChatID. Pass 0 to return every protected channel regardless of
	// controlling chat.
	ListChannels(ctx context.Context, commandChatID int64) ([]ChannelView, error)
	// GetChannel returns a single channel by ID.
	GetChannel(ctx context.Context, channelID int64) (ChannelView, error)
	// AddChannel registers a new protected channel. commandChatID is the
	// controlling chat it is attached to.
	AddChannel(ctx context.Context, channelID, commandChatID int64, autoScan, autoClean, allowClean bool) error
	// RemoveChannel detaches channelID from commandChatID (and deletes it
	// entirely when no controlling chats remain). Returns an error if the chat
	// does not control the channel.
	RemoveChannel(ctx context.Context, channelID, commandChatID int64) error
	// SetChannelFlags updates the AutoScan/AutoClean/AllowClean flags for a
	// channel, persists the change and keeps the periodic ticker in sync.
	SetChannelFlags(ctx context.Context, channelID int64, autoScan, autoClean, allowClean bool) error
	// ListChannelUsers fetches and classifies the members of channelID.
	ListChannelUsers(ctx context.Context, channelID int64) (UsersView, error)
	// TriggerScan enqueues a manual scan of channelID; reportChannelID is where
	// the report is sent (typically the controlling chat).
	TriggerScan(ctx context.Context, channelID, reportChannelID int64) error
	// TriggerClean enqueues a manual clean of channelID.
	TriggerClean(ctx context.Context, channelID, reportChannelID int64) error
	// GetStatus returns the aggregated bot/userbot identity and channel list.
	GetStatus(ctx context.Context) (StatusView, error)
	// RefreshRights re-resolves the bot's rights and channel titles from
	// Telegram. Useful after manual changes in the Telegram client.
	RefreshRights(ctx context.Context) error
}
