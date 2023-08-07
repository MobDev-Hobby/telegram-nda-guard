package report_processor_send_admin_with_telegram_bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report report_processor.AccessReport,
) {

	messages := make([]string, 1)
	message := strings.Builder{}

	message.WriteString(
		fmt.Sprintf(
			"Channel participants check report for channel id: <b>%d</b>\n good users: <b>%d</b>, bad users: <b>%d</b>, can't check <b>%d</b> users.\n",
			report.ChannelId,
			len(report.AllowedUsers),
			len(report.DeniedUsers),
			len(report.UnknownUsers),
		),
	)

	if len(report.DeniedUsers) > 0 {
		message.WriteString("\n<b>Not allowed users:</b>\n")
		for _, user := range report.DeniedUsers {
			lastname := ""
			if len(user.LastName) > 0 {
				lastname = fmt.Sprintf(" %s", user.LastName)
			}
			userLink := ""
			if len(user.Username) > 0 {
				userLink = fmt.Sprintf(
					" <a href=\"https://t.me/%s\">%s%s</a>",
					user.Username,
					user.Firstname,
					lastname,
				)
			}else if len(user.Phone) > 0{
				userLink = fmt.Sprintf(
					" <a href=\"https://t.me/+%s\">%s%s</a>",
					user.Phone,
					user.Firstname,
					lastname,
				)
			} else{
				userLink = fmt.Sprintf(
					" <a href=\"tg://user?id=%d\">%s%s</a>",
					user.ID,
					user.Firstname,
					lastname,
				)
			}
			chunk := fmt.Sprintf(
				"• %s\n",
				userLink,
			)

			if message.Len()+len(chunk) > 4000 {
				messages = append(messages, message.String())
				message.Reset()
			}
			message.WriteString(chunk)
		}
	}
	if len(report.UnknownUsers) > 0 {
		message.WriteString("\n<b>Can't check users:</b>\n")
		for _, user := range report.UnknownUsers {
			lastname := ""
			if len(user.LastName) > 0 {
				lastname = fmt.Sprintf(" %s", user.LastName)
			}
			userLink := ""
			if len(user.Username) > 0 {
				userLink = fmt.Sprintf(
					" <a href=\"https://t.me/%s\">%s%s</a>",
					user.Username,
					user.Firstname,
					lastname,
				)
			}else if len(user.Phone) > 0{
				userLink = fmt.Sprintf(
					" <a href=\"https://t.me/+%s\">%s%s</a>",
					user.Phone,
					user.Firstname,
					lastname,
				)
			} else{
				userLink = fmt.Sprintf(
					" <a href=\"tg://user?id=%d\">%s%s</a>",
					user.ID,
					user.Firstname,
					lastname,
				)
			}
			chunk := fmt.Sprintf(
				"• %s\n",
				userLink,
			)

			if message.Len()+len(chunk) > 4000 {
				messages = append(messages, message.String())
				message.Reset()
			}
			message.WriteString(chunk)
		}
	}
	messages = append(messages, message.String())
	message.Reset()

	for _, chatId := range d.reportChatIds {
		for _, message := range messages {
			if len(message) == 0 {
				continue
			}

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
}
