// Package authorizer provides a default (hybrid) implementation of
// scanner.Authorizer. It is kept in its own package so the controller does not
// depend on a concrete authorization strategy and consumers can substitute
// their own (e.g. an SSO/JWT-backed authorizer) via scanner.WithAuthorizer.
package authorizer

import (
	"context"
	"errors"
	"sync"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// ChatAdminLister returns the user IDs of the administrators (owner + admins)
// of the given chat. The bundled telegram/bots/bot.Domain satisfies this.
type ChatAdminLister interface {
	GetChatAdministrators(ctx context.Context, chatID int64) ([]int64, error)
}

// Logger mirrors the project-wide logger contract (intentionally duplicated
// per-package rather than shared, matching the rest of the codebase).
type Logger interface {
	Errorf(template string, args ...any)
	Debugf(template string, args ...any)
}

// HybridAuthorizer allows a command when ANY of the following holds:
//
//   - the sender's user ID is in the explicit allowlist;
//   - the sender's user ID is the configured owner;
//   - the sender is an administrator of the chat the command originated from
//     (only when RequireAdmin is true).
//
// When RequireAdmin is false and neither the allowlist nor owner matches, the
// command is denied. This keeps authorization explicit and auditable.
//
// The allowlist and owner are checked first (cheap, no I/O); the admin check is
// performed last because it issues a Telegram API call.
type HybridAuthorizer struct {
	bot          ChatAdminLister
	ownerUserID  int64
	allowUserIDs []int64
	requireAdmin bool
	log          Logger

	adminCacheTTL time.Duration
	adminCacheMu  sync.Mutex
	adminCache    map[int64]adminCacheEntry
}

type adminCacheEntry struct {
	admins  []int64
	fetched time.Time
}

// Option configures a HybridAuthorizer.
type Option func(*HybridAuthorizer)

// WithOwner sets the owner user ID that is always allowed.
func WithOwner(ownerUserID int64) Option {
	return func(h *HybridAuthorizer) {
		h.ownerUserID = ownerUserID
	}
}

// WithAllowlist adds explicit allowed user IDs in addition to the owner.
func WithAllowlist(userIDs []int64) Option {
	return func(h *HybridAuthorizer) {
		h.allowUserIDs = append(h.allowUserIDs, userIDs...)
	}
}

// WithRequireAdmin enables the "administrator of the originating chat" check.
// When enabled, a sender who is an admin of the chat the command came from is
// also allowed. Without it, only the owner and allowlist are honored.
func WithRequireAdmin() Option {
	return func(h *HybridAuthorizer) {
		h.requireAdmin = true
	}
}

// WithLogger injects a logger. Optional; a no-op logger is used by default.
func WithLogger(log Logger) Option {
	return func(h *HybridAuthorizer) {
		h.log = log
	}
}

// WithAdminCacheTTL sets how long a chat's administrator list is reused before
// asking Telegram again (default 1 minute; 0 disables caching). A demoted
// administrator keeps access for at most this long.
func WithAdminCacheTTL(ttl time.Duration) Option {
	return func(h *HybridAuthorizer) {
		h.adminCacheTTL = ttl
	}
}

// noopLogger discards all output.
type noopLogger struct{}

func (noopLogger) Errorf(string, ...any) {}
func (noopLogger) Debugf(string, ...any) {}

// New creates a HybridAuthorizer backed by bot. Pass functional options to
// configure owner, allowlist and admin enforcement.
func New(bot ChatAdminLister, opts ...Option) *HybridAuthorizer {
	h := &HybridAuthorizer{
		bot:           bot,
		log:           noopLogger{},
		adminCacheTTL: time.Minute,
		adminCache:    make(map[int64]adminCacheEntry),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Authorize implements scanner.Authorizer (the Telegram-side interface). It
// extracts the sender and chat IDs from the update and delegates to the
// transport-neutral authorizeIDs.
func (h *HybridAuthorizer) Authorize(ctx context.Context, update *guard.Update) (bool, error) {
	if update == nil {
		return false, errors.New("authorize: nil update")
	}
	if update.Message != nil {
		return h.authorizeIDs(ctx, update.Message.User.ID, update.Message.ChatID)
	}
	if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		// CallbackQuery.Message.User is the author of the message that carries
		// the button, i.e. the bot. The presser is CallbackQuery.From.
		if update.CallbackQuery.From.ID == 0 {
			return false, errors.New("authorize: callback without sender")
		}
		return h.authorizeIDs(ctx, update.CallbackQuery.From.ID, update.CallbackQuery.Message.ChatID)
	}
	return false, errors.New("authorize: update has no message")
}

// AuthenticateAndAuthorize implements scanner.WebAuthenticator. It is the
// transport-neutral entry point used by non-Telegram surfaces (e.g. the web
// API): callerID is the verified Telegram user ID obtained from the login
// widget, and scopeChatID is the chat/channel the operation targets (0 when
// irrelevant, which disables the admin-check branch).
func (h *HybridAuthorizer) AuthenticateAndAuthorize(ctx context.Context, callerID, scopeChatID int64) (bool, error) {
	return h.authorizeIDs(ctx, callerID, scopeChatID)
}

// authorizeIDs is the single decision core shared by both transports. It
// resolves whether callerID may act, optionally checking that they are an
// administrator of scopeChatID when RequireAdmin is set and scopeChatID != 0.
func (h *HybridAuthorizer) authorizeIDs(ctx context.Context, callerID, scopeChatID int64) (bool, error) {
	// 1. Owner and explicit allowlist always win.
	if h.isPrivileged(callerID) {
		return true, nil
	}
	// 3. Optional: caller is an admin of the originating/target chat. Skipped
	//    when scopeChatID is 0 (e.g. web calls without a chat context).
	if h.requireAdmin && scopeChatID != 0 {
		return h.isChatAdmin(ctx, callerID, scopeChatID), nil
	}
	return false, nil
}

// AuthorizeChannel implements the channel-scoped check used by the Mini App:
// the owner and the allowlist may manage every channel, anyone else only the
// channels they administer. Unlike authorizeIDs it does not depend on
// WithRequireAdmin: administering a channel is what entitles a user to manage
// its protection.
func (h *HybridAuthorizer) AuthorizeChannel(ctx context.Context, callerID, channelID int64) (bool, error) {
	if h.isPrivileged(callerID) {
		return true, nil
	}
	if channelID == 0 {
		return false, nil
	}
	return h.isChatAdmin(ctx, callerID, channelID), nil
}

// IsPrivileged reports whether callerID is the owner or on the allowlist.
func (h *HybridAuthorizer) IsPrivileged(_ context.Context, callerID int64) bool {
	return h.isPrivileged(callerID)
}

func (h *HybridAuthorizer) isPrivileged(callerID int64) bool {
	if h.ownerUserID != 0 && callerID == h.ownerUserID {
		return true
	}
	for _, id := range h.allowUserIDs {
		if id == callerID {
			return true
		}
	}
	return false
}

// isChatAdmin fails closed: when the admin list can't be fetched the caller is
// not treated as an admin.
func (h *HybridAuthorizer) isChatAdmin(ctx context.Context, callerID, chatID int64) bool {
	admins, err := h.chatAdmins(ctx, chatID)
	if err != nil {
		h.log.Errorf("authorize: can't get chat admins for %d: %s", chatID, err)
		return false
	}
	for _, id := range admins {
		if id == callerID {
			return true
		}
	}
	return false
}

func (h *HybridAuthorizer) chatAdmins(ctx context.Context, chatID int64) ([]int64, error) {
	if h.adminCacheTTL > 0 {
		h.adminCacheMu.Lock()
		entry, ok := h.adminCache[chatID]
		h.adminCacheMu.Unlock()
		if ok && time.Since(entry.fetched) < h.adminCacheTTL {
			return entry.admins, nil
		}
	}
	admins, err := h.bot.GetChatAdministrators(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if h.adminCacheTTL > 0 {
		h.adminCacheMu.Lock()
		h.adminCache[chatID] = adminCacheEntry{admins: admins, fetched: time.Now()}
		h.adminCacheMu.Unlock()
	}
	return admins, nil
}
