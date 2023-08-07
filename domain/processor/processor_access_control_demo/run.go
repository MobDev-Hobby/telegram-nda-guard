package processor_access_control_demo

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
)

func (d *Domain) Run(
	ctx context.Context,
) error {

	err := d.telegramBot.Run(ctx)
	if err != nil {
		return fmt.Errorf("can't run telegram bot: %v", err)
	}

	d.telegramBot.GetBot().RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			if update.Message != nil &&
				strings.HasPrefix(update.Message.Text, "/id") {
				return true
			}
			return false
		},
		d.getIdHandler,
	)

	sessionStorage := d.sessionStorageProvider.GetStorage(userBotName)

	if d.adminUserChatId != 0 {
		d.log.Debugf("Run userbot for user %d", d.adminUserChatId)
		d.userBot = d.userBotProvider.NewBot(
			ctx,
			sessionStorage,
			d.NewBotAuthFlow(d.adminUserChatId),
		)
	} else {
		d.log.Debugf("No userbot admin found want new")

		d.telegramBot.GetBot().RegisterHandlerMatchFunc(
			func(update *models.Update) bool {
				if update.Message != nil &&
					update.Message.Chat.Type == "private" &&
					strings.HasPrefix(update.Message.Text, "/admin") {
					return true
				}
				return false
			},
			d.getSetAdminHandlerWithCallback(
				func() {
					d.log.Debugf("Run userbot for user %d", d.adminUserChatId)
					d.userBot = d.userBotProvider.NewBot(
						ctx,
						sessionStorage,
						d.NewBotAuthFlow(d.adminUserChatId),
					)
				},
			),
		)
	}

	d.RunUserAccessChecker(ctx)
	
	return nil
}
