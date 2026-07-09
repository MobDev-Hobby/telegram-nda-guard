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
// normalization (-100 prefix) lives in the wrapped restrictor, so the kicker
// deals only in logical channel ids.
func (d *Domain) cleanUser(
	ctx context.Context,
	channel guard.ChannelInfo,
	user guard.User,
) bool {

	// A ban is issued when we either keep the user banned or want message
	// cleanup (ban+unban is how Telegram deletes the user's recent messages).
	wasBanned := false
	if d.keepBanned || d.cleanMessages {
		if err := d.botClient.Ban(ctx, channel.ID, user.ID, d.cleanMessages); err != nil {
			d.log.Errorf("can't ban user %s: %s", user.Username, err)
			return false
		}
		wasBanned = true
	}

	if !d.keepBanned {
		// OnlyIfBanned must reflect whether the ban step ran:
		//   - keepBanned=false && cleanMessages=true  -> ban-then-unban to
		//     clean messages; only_if_banned=true avoids a spurious error if
		//     the ban did not register in time.
		//   - keepBanned=false && cleanMessages=false -> the ban step was
		//     skipped, so we must call unban with only_if_banned=false to
		//     actually remove the member (this is the "kick" path). Using
		//     true here would be a no-op and leave the user in the chat.
		if err := d.botClient.Unban(ctx, channel.ID, user.ID, wasBanned); err != nil {
			d.log.Errorf("can't unban user %s: %s", user.Username, err)
			return false
		}
	}

	return true
}
