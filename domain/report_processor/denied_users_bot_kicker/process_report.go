package denied_users_bot_kicker

import (
	"context"
	"fmt"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report report_processor.AccessReport,
) {

	usersToClean := report.DeniedUsers
	cleanedUsers := 0
	if d.cleanUnknown {
		usersToClean = append(usersToClean, report.UnknownUsers...)
	}

	for _, user := range usersToClean {
		success := true
		if d.keepBanned || d.cleanMessages {
			successBanned, err := d.botClient.BanChatMember(
				ctx,
				&bot.BanChatMemberParams{
					ChatID:         report.ChannelId,
					UserID:         user.ID,
					RevokeMessages: d.cleanMessages,
				},
			)
			if err != nil {
				d.log.Errorf("can't send ban user: %s. Error: %s", user.Username, err.Error())
			}
			success = success && successBanned
		}

		if !d.keepBanned {
			successKicked, err := d.botClient.UnbanChatMember(
				ctx,
				&bot.UnbanChatMemberParams{
					ChatID:       report.ChannelId,
					UserID:       user.ID,
					OnlyIfBanned: false,
				},
			)
			if err != nil {
				d.log.Errorf("can't send ban user: %s. Error: %s", user.Username, err.Error())
			}
			success = success && successKicked
		}
		if success {
			cleanedUsers++
		}
	}

	message := fmt.Sprintf(
		"Good users: %d. Tried to kick %d bad users, kicked %d bad users. Keep banned: %b. Clean messages: %b. Clean unknown: %b",
		len(report.AllowedUsers),
		len(usersToClean),
		cleanedUsers,
		d.keepBanned,
		d.cleanMessages,
		d.cleanUnknown,
	)
	for _, chatId := range d.reportChatIds {
		_, err := d.botClient.SendMessage(
			ctx,
			&bot.SendMessageParams{
				ChatID:                chatId,
				Text:                  message,
				ParseMode:             models.ParseModeHTML,
				DisableWebPagePreview: true,
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s. Message text: %s", err, message)
		}
		d.log.Debugf(message)
	}
}
