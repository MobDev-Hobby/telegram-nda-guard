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
		// successfully kicked users in the "Kicked N/M" report.
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
// rate-limited and to handle Telegram FLOOD_WAIT (429) internally; chat-id
// normalization (-100 prefix) and OnlyIfBanned handling live in the wrapped
// restrictor, so the kicker deals only in logical channel ids.
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
		// OnlyIfBanned:true is applied inside the restrictor client, so when
		// the ban step did not take effect the unban is a no-op instead of an
		// error. Previously OnlyIfBanned:false produced a misleading
		// "USER_NOT_PARTICIPANT" error logged (wrongly) as "can't send ban".
		if err := d.botClient.Unban(ctx, channel.ID, user.ID); err != nil {
			d.log.Errorf("can't unban user %s: %s", user.Username, err)
			return false
		}
	}

	return true
}
