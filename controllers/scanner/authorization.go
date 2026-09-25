package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// allowAllAuthorizer permits every update. It is the default when no
// Authorizer is configured, preserving the pre-authorization behavior where any
// member of a controlling chat could run commands. Operators who want to
// restrict access should pass a real Authorizer via WithAuthorizer.
type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Authorize(_ context.Context, _ *guard.Update) (bool, error) {
	return true, nil
}

// authorizer returns the configured Authorizer, falling back to an allow-all
// implementation when none was set. This keeps Run() backwards-compatible.
func (d *Domain) resolvedAuthorizer() Authorizer {
	if d.authorizer != nil {
		return d.authorizer
	}
	return allowAllAuthorizer{}
}

// authorize checks whether the sender of update may run a protected command.
// Returns true when authorized (or when authorization is not enforced). On a
// denial it logs at debug level so operators can see why a command was ignored.
func (d *Domain) authorize(ctx context.Context, update *guard.Update) bool {
	ok, err := d.resolvedAuthorizer().Authorize(ctx, update)
	if err != nil {
		d.log.Errorf("authorize denied command: %s", err)
		return false
	}
	if !ok {
		d.log.Debugf("authorize denied command from update")
	}
	return ok
}

// requireAuth wraps a command callback so that it only runs when the sender is
// authorized. It is the single chokepoint used when registering handlers, so
// every protected command goes through authorization uniformly.
func (d *Domain) requireAuth(
	callback func(ctx context.Context, update *guard.Update),
) func(ctx context.Context, update *guard.Update) {
	return func(ctx context.Context, update *guard.Update) {
		if !d.authorize(ctx, update) {
			return
		}
		callback(ctx, update)
	}
}

// requireLinkedChannel wraps a handler whose payload may carry a channel id
// ("/cmd <id> ..."). When an id is present, the update must come from a chat
// that controls that channel; otherwise any chat the bot is in could read the
// member list of, reconfigure or detach any protected channel by guessing its
// id. Payloads without an id pass through (the handler lists channels scoped
// to the originating chat itself).
func (d *Domain) requireLinkedChannel(
	callback func(ctx context.Context, update *guard.Update),
) func(ctx context.Context, update *guard.Update) {
	return func(ctx context.Context, update *guard.Update) {
		var chatID int64
		var payload, callbackID string
		switch {
		case update.CallbackQuery != nil && update.CallbackQuery.Message != nil:
			chatID = update.CallbackQuery.Message.ChatID
			payload = update.CallbackQuery.Data
			callbackID = update.CallbackQuery.ID
		case update.Message != nil:
			chatID = update.Message.ChatID
			payload = update.Message.Text
		default:
			return
		}
		if channelID, ok := parseChannelArg(payload); ok && !d.isChannelLinkedToControlChat(chatID, channelID) {
			d.log.Warnf("chat %d tried to operate on unlinked channel %d", chatID, channelID)
			if callbackID != "" {
				d.telegramBot.CallbackResponse(ctx, guard.CallbackResponse{
					ID: callbackID, Text: "This channel is not controlled from this chat", ShowAlert: true,
				})
			}
			return
		}
		callback(ctx, update)
	}
}

// requireAuthOrPrivateEmployee lets /add run in a private chat for anyone who
// passes the access checker, on top of the regular authorization. That is
// safe: Telegram's chat picker only offers chats where the user is an
// administrator with the right to ban, so a user can only protect chats they
// already administer.
func (d *Domain) requireAuthOrPrivateEmployee(
	callback func(ctx context.Context, update *guard.Update),
) func(ctx context.Context, update *guard.Update) {
	return func(ctx context.Context, update *guard.Update) {
		if msg := update.Message; msg != nil && msg.ChatType == guard.ChatTypePrivate && d.defaultAccessChecker != nil {
			user := msg.User
			ok, err := d.defaultAccessChecker.HasAccess(ctx, &user)
			if err == nil && ok {
				callback(ctx, update)
				return
			}
		}
		if !d.authorize(ctx, update) {
			return
		}
		callback(ctx, update)
	}
}
