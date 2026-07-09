package kicker

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
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

	for _, user := range usersToClean {
		// Count success per-user. Previously `success` was accumulated across
		// iterations (`success = success && ...`), so once any user failed the
		// counter stayed false for every subsequent user, undercounting even
		// successfully kicked users.
		if d.cleanUser(ctx, report.Channel, user) {
			cleanedUsers++
		}
	}

	message := fmt.Sprintf(
		"<b>Clean report for %s %s</b>"+
			"\n\n<b>Users:</b>"+
			"\n• Good: <b>%d</b>"+
			"\n• Unknown: <b>%d</b>"+
			"\n• Bad: <b>%d</b>"+
			"\n\nKicked <b>%d/%d</b> bad users.\n\n"+
			"<b>Settings:</b>\n"+
			"• Keep banned: <b>%t</b>. \n"+
			"• Clean messages: <b>%t</b>. \n"+
			"• Clean unknown: <b>%t</b>",
		guard.ChatTypeNoun(report.Channel.Type),
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
		if err := d.botClient.SendReport(ctx, chatID, message); err != nil {
			d.log.Errorf("can't send message: %s. Message text: %s", err, message)
		}
	}
	d.log.Debugf(message)
}

// cleanUser bans then (optionally) unbans a single user. Returns true when the
// user was removed from the channel. The bot client is expected to be
// rate-limited and to handle Telegram FLOOD_WAIT (429) internally.
func (d *Domain) cleanUser(
	ctx context.Context,
	channel guard.ChannelInfo,
	user guard.User,
) bool {

	if d.keepBanned || d.cleanMessages {
		if err := d.botClient.Ban(ctx, channel.ID, user.ID, d.cleanMessages); err != nil {
			d.log.Errorf("can't ban user %s: %s", user.Username, err)
			return false
		}
	}

	if !d.keepBanned {
		// The previous implementation called Unban with OnlyIfBanned:false,
		// so when the ban step did not succeed the unban still fired and
		// produced a misleading "USER_NOT_PARTICIPANT" error logged as "ban".
		// OnlyIfBanned handling now lives in the rate-limited client.
		if err := d.botClient.Unban(ctx, channel.ID, user.ID); err != nil {
			d.log.Errorf("can't unban user %s: %s", user.Username, err)
			return false
		}
	}

	return true
}
