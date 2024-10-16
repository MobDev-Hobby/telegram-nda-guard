package kicker

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report processors.AccessReport,
) {

	usersToClean := report.DeniedUsers
	cleanedUsers := 0
	if d.cleanUnknown {
		usersToClean = append(usersToClean, report.UnknownUsers...)
	}

	commandChatID := report.Channel.ID
	if commandChatID > 0 {
		commandChatID, _ = strconv.ParseInt(
			fmt.Sprintf("-100%d", report.Channel.ID),
			10,
			64,
		)
	}

	success := true
	for _, user := range usersToClean {
		chanToBanUser := commandChatID
		if report.Channel.MigratedTo != nil {
			chanToBanUser = *report.Channel.MigratedTo
		}

		if d.keepBanned || d.cleanMessages {
			successBanned, err := d.botClient.BanChatMember(
				ctx,
				&bot.BanChatMemberParams{
					ChatID:         chanToBanUser,
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
					ChatID:       chanToBanUser,
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
		"<b>Clean report for chat %s</b>"+
			"\n\n<b>Users:</b>"+
			"\n• Good: <b>%d</b>"+
			"\n• Unknown: <b>%d</b>"+
			"\n• Bad: <b>%d</b>"+
			"\n\nKicked <b>%d/%d</b> bad users.\n\n"+
			"<b>Settings:</b>\n"+
			"• Keep banned: <b>%t</b>. \n"+
			"• Clean messages: <b>%t</b>. \n"+
			"• Clean unknown: <b>%t</b>",
		report.Channel.Title,
		len(report.AllowedUsers),
		len(report.UnknownUsers),
		len(report.DeniedUsers),
		cleanedUsers,
		len(usersToClean),
		d.keepBanned,
		d.cleanMessages,
		d.cleanUnknown,
	)

	for _, chatID := range report.ReportChannels {
		_, err := d.botClient.SendMessage(
			ctx,
			&bot.SendMessageParams{
				ChatID:    chatID,
				Text:      message,
				ParseMode: models.ParseModeHTML,
				LinkPreviewOptions: &models.LinkPreviewOptions{
					IsDisabled: utils.Ptr(true),
				},
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s. Message text: %s", err, message)
		}
	}
	d.log.Debugf(message)
}
