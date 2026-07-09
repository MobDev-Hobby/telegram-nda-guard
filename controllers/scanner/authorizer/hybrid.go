// Package authorizer provides a default (hybrid) implementation of
// scanner.Authorizer. It is kept in its own package so the controller does not
// depend on a concrete authorization strategy and consumers can substitute
// their own (e.g. an SSO/JWT-backed authorizer) via scanner.WithAuthorizer.
package authorizer

import (
	"context"
	"errors"

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
	bot           ChatAdminLister
	ownerUserID   int64
	allowUserIDs  []int64
	requireAdmin  bool
	log           Logger
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

// noopLogger discards all output.
type noopLogger struct{}

func (noopLogger) Errorf(string, ...any) {}
func (noopLogger) Debugf(string, ...any) {}

// New creates a HybridAuthorizer backed by bot. Pass functional options to
// configure owner, allowlist and admin enforcement.
func New(bot ChatAdminLister, opts ...Option) *HybridAuthorizer {
	h := &HybridAuthorizer{
		bot: bot,
		log: noopLogger{},
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
		return h.authorizeIDs(ctx, update.CallbackQuery.Message.User.ID, update.CallbackQuery.Message.ChatID)
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
	// 1. Owner always wins.
	if h.ownerUserID != 0 && callerID == h.ownerUserID {
		return true, nil
	}
	// 2. Explicit allowlist.
	for _, id := range h.allowUserIDs {
		if id == callerID {
			return true, nil
		}
	}
	// 3. Optional: caller is an admin of the originating/target chat. Skipped
	//    when scopeChatID is 0 (e.g. web calls without a chat context).
	if h.requireAdmin && scopeChatID != 0 {
		admins, err := h.bot.GetChatAdministrators(ctx, scopeChatID)
		if err != nil {
			h.log.Errorf("authorize: can't get chat admins for %d: %s", scopeChatID, err)
			return false, nil
		}
		for _, id := range admins {
			if id == callerID {
				return true, nil
			}
		}
	}
	return false, nil
}
