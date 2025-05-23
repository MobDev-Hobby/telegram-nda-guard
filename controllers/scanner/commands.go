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
		d.retryHandler,
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
		d.checkChannelHandler,
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
		d.processScanCallbackHandler,
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
		d.cleanChannelHandler,
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
		d.processCleanCallbackHandler,
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
		d.ListChannelsHandler,
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
			d.AddChannelHandler,
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

	if d.adminUserChatID == 0 {
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
