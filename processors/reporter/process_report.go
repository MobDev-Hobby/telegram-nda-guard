package reporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report processors.AccessReport,
) {

	messages := make([]string, 0, 1)
	message := strings.Builder{}

	_, err := message.WriteString(
		fmt.Sprintf(
			"<b>Scan report for chat %s</b>"+
				"\n\n<b>Users:</b>"+
				"\n• Good: <b>%d</b>"+
				"\n• Unknown: <b>%d</b>"+
				"\n• Bad: <b>%d</b>\n",
			report.Channel.Title,
			len(report.AllowedUsers),
			len(report.UnknownUsers),
			len(report.DeniedUsers),
		),
	)
	if err != nil {
		d.log.Errorf("ProcessReport write message failed: %v", err)
	}

	for _, reportChunk := range []struct {
		title string
		users []guard.User
	}{
		{title: "\n<b>Not allowed users:</b>\n", users: report.DeniedUsers},
		{title: "\n<b>Can't check users:</b>\n", users: report.UnknownUsers},
	} {
		if len(reportChunk.users) > 0 {
			_, err := message.WriteString(reportChunk.title)
			if err != nil {
				d.log.Errorf("ProcessReport write message failed: %v", err)
			}
			for _, chunk := range d.listUsers(reportChunk.users) {
				if message.Len()+len(chunk) > 4000 {
					messages = append(messages, message.String())
					message.Reset()
				}
				_, err := message.WriteString(chunk)
				if err != nil {
					d.log.Errorf("ProcessReport write message failed: %v", err)
				}
			}
		}
	}

	messages = append(messages, message.String())
	message.Reset()

	d.sendReportForChannel(ctx, report.Channel.ID, report.ReportChannels, messages)
}

func (d *Domain) listUsers(users []guard.User) []string {
	userLinks := make([]string, 0, len(users))
	for _, user := range users {
		userLinks = append(
			userLinks, fmt.Sprintf(
				"• %s\n",
				d.getUserLink(user),
			),
		)
	}
	return userLinks
}

func (d *Domain) getUserLink(user guard.User) string {
	lastname := ""
	if len(user.LastName) > 0 {
		lastname = fmt.Sprintf(" %s", user.LastName)
	}

	if len(user.Username) > 0 {
		return fmt.Sprintf(
			" <a href=\"https://t.me/%s\">%s%s</a>",
			user.Username,
			user.FirstName,
			lastname,
		)
	} else if user.Phone != nil {
		return fmt.Sprintf(
			" <a href=\"https://t.me/+%s\">%s%s</a>",
			*user.Phone,
			user.FirstName,
			lastname,
		)
	}
	return fmt.Sprintf(
		" <a href=\"tg://user?id=%d\">%s%s</a>",
		user.ID,
		user.FirstName,
		lastname,
	)
}

func (d *Domain) sendReportForChannel(ctx context.Context, channelID int64, reportChannels []int64, messages []string) {
	for _, chatID := range reportChannels {
		for _, message := range messages {
			if len(messages) == 0 {
				continue
			}

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
			d.log.Debugf(message)
		}
	}
}
