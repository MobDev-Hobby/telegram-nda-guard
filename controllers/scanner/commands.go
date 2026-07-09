package scanner

import (
	"context"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) setupCommands(ctx context.Context) {
	d.log.Debugf("Register /id handler")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message != nil &&
				strings.HasPrefix(update.Message.Text, "/id") {

				return true
			}
			return false
		},
		d.IDHandler,
	)

	d.log.Debugf("Register /retry handler")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message != nil &&
				strings.HasPrefix(update.Message.Text, "/retry") {

				return true
			}
			return false
		},
		// /retry forces a userbot reconnection. Without authorization any
		// user could spam it to drive a re-init loop (DoS / log spam), so
		// gate it on the configured admin chat.
		d.requireAdmin(d.retryHandler),
	)

	d.log.Debugf("Register /start and /help handlers")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message != nil &&
				(strings.HasPrefix(update.Message.Text, "/start") ||
					strings.HasPrefix(update.Message.Text, "/help")) {

				return true
			}
			return false
		},
		d.startHandler,
	)

	d.log.Debugf("Register /scan handler")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message != nil &&
				strings.HasPrefix(update.Message.Text, "/scan") {

				return true
			}
			return false
		},
		d.requireAuth(d.checkChannelHandler),
	)
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.CallbackQuery != nil && update.CallbackQuery.Message != nil &&
					strings.HasPrefix(update.CallbackQuery.Data, "/scan") {

					return true
				}
				return false
			},
			d.requireAuth(d.processScanCallbackHandler),
		)

		d.log.Debugf("Register /clean handler")
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.Message != nil &&
					strings.HasPrefix(update.Message.Text, "/clean") {

					return true
				}
				return false
			},
			d.requireAuth(d.cleanChannelHandler),
		)
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.CallbackQuery != nil && update.CallbackQuery.Message != nil &&
					strings.HasPrefix(update.CallbackQuery.Data, "/clean") {

					return true
				}
				return false
			},
			d.requireAuth(d.processCleanCallbackHandler),
		)

		d.log.Debugf("Register /list handler")
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.Message != nil &&
					strings.HasPrefix(update.Message.Text, "/list") {

					return true
				}
				return false
			},
			d.requireAuth(d.ListChannelsHandler),
		)

	d.log.Debugf("Register /settings handlers")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.Message != nil &&
				strings.HasPrefix(update.Message.Text, "/settings") {

				return true
			}
			return false
		},
		d.SettingsHandler,
	)
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.CallbackQuery != nil && update.CallbackQuery.Message != nil &&
				strings.HasPrefix(update.CallbackQuery.Data, "/settings") {

				return true
			}
			return false
		},
		d.SettingsChannelHandler,
	)

	d.log.Debugf("Register /setflag handler")
	d.telegramBot.RegisterHandler(
		ctx,
		func(update *guard.Update) bool {
			if update.CallbackQuery != nil && update.CallbackQuery.Message != nil &&
				strings.HasPrefix(update.CallbackQuery.Data, "/setflag") {

				return true
			}
			return false
		},
		d.ToggleFlagHandler,
	)

	if d.defaultCleanProcessor != nil && d.defaultAccessChecker != nil {
		d.log.Debugf("Register /add handler")
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.Message != nil &&
					strings.HasPrefix(update.Message.Text, "/add") {

					return true
				}
				return false
			},
			d.requireAuth(d.AddChannelHandler),
		)
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.Message != nil && update.Message.ChatShared != nil {
					return true
				}
				return false
			},
			d.AddChannelCallbackHandler,
		)
	}

	// /admin bootstrap is only useful while no owner is configured. The
	// Domain default is adminUserChatID == -1 and WithOwnerChatID requires
	// a positive id, so the previous "== 0" gate made this route
	// unreachable. Use "<= 0" to also cover the default sentinel.
	if d.adminUserChatID <= 0 {
		d.log.Debugf("Register /admin <code> handler")
		d.telegramBot.RegisterHandler(
			ctx,
			func(update *guard.Update) bool {
				if update.Message != nil &&
					update.Message.ChatType == "private" &&
					strings.HasPrefix(update.Message.Text, "/admin") {

					return true
				}
				return false
			},
			d.getSetAdminHandlerWithCallback(
				func() {
					d.log.Debugf("New admin chat %d", d.adminUserChatID)
				},
			),
		)
	}
}

// requireAdmin wraps a handler so it only runs when the update originates
// from the configured admin chat. Previously /retry (and the removed /call)
// had no authorization, letting any user trigger privileged bot/userbot
// actions. Callback-based command handlers that operate on protected
// channels additionally validate the target channel separately.
func (d *Domain) requireAdmin(
	handler func(ctx context.Context, update *guard.Update),
) func(ctx context.Context, update *guard.Update) {
	return func(ctx context.Context, update *guard.Update) {
		var chatID int64
		switch {
		case update.Message != nil:
			chatID = update.Message.ChatID
		case update.CallbackQuery != nil && update.CallbackQuery.Message != nil:
			chatID = update.CallbackQuery.Message.ChatID
		default:
			return
		}
		if chatID != d.adminUserChatID {
			d.log.Warnf("unauthorized command attempt from chat %d", chatID)
			return
		}
		handler(ctx, update)
	}
}
