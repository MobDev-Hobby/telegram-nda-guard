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

	if d.adminUserChatID != 0 {
		d.log.Debugf("Run userbot for user %d", d.adminUserChatID)
		d.runUserBot(ctx)
		d.log.Debugf("User bot launched")
	} else {
		d.log.Debugf("No userbot admin found want new")
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
					d.log.Debugf("Run userbot for user %d", d.adminUserChatID)
					d.runUserBot(ctx)
					d.log.Debugf("User bot launched")
				},
			),
		)
	}
}
