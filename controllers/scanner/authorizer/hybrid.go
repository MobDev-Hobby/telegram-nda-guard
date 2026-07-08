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

// Authorize implements scanner.Authorizer.
func (h *HybridAuthorizer) Authorize(ctx context.Context, update *guard.Update) (bool, error) {
	if update == nil || update.Message == nil {
		// Callbacks carry the originating message; authorize on that.
		if update != nil && update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
			return h.authorizeMessage(ctx, update.CallbackQuery.Message)
		}
		return false, errors.New("authorize: update has no message")
	}
	return h.authorizeMessage(ctx, update.Message)
}

func (h *HybridAuthorizer) authorizeMessage(ctx context.Context, msg *guard.MessageReceived) (bool, error) {
	senderID := msg.User.ID

	// 1. Owner always wins.
	if h.ownerUserID != 0 && senderID == h.ownerUserID {
		return true, nil
	}
	// 2. Explicit allowlist.
	for _, id := range h.allowUserIDs {
		if id == senderID {
			return true, nil
		}
	}
	// 3. Optional: sender is an admin of the originating chat.
	if h.requireAdmin {
		admins, err := h.bot.GetChatAdministrators(ctx, msg.ChatID)
		if err != nil {
			h.log.Errorf("authorize: can't get chat admins for %d: %s", msg.ChatID, err)
			return false, nil
		}
		for _, id := range admins {
			if id == senderID {
				return true, nil
			}
		}
	}
	return false, nil
}
