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

	// Per-channel options override the process-wide defaults when present.
	keepBanned := d.keepBanned
	cleanMessages := d.cleanMessages
	cleanUnknown := d.cleanUnknown
	if report.CleanOptions != nil {
		keepBanned = report.CleanOptions.KeepBanned
		cleanMessages = report.CleanOptions.CleanMessages
		cleanUnknown = report.CleanOptions.CleanUnknown
	}

	usersToClean := report.DeniedUsers
	cleanedUsers := 0
	if cleanUnknown {
		usersToClean = append(usersToClean, report.UnknownUsers...)
	}

	// The channel the user should be restricted in. When a chat migrated to a
	// supergroup, the new ID is authoritative; the UserRestrictor is then
	// responsible for any transport-specific normalization of that ID.
	channelToBan := report.Channel.ID
	if report.Channel.MigratedTo != nil {
		channelToBan = *report.Channel.MigratedTo
	}

	success := true
	for _, user := range usersToClean {
		if keepBanned || cleanMessages {
			if err := d.restrictor.Ban(ctx, channelToBan, user.ID, cleanMessages); err != nil {
				d.log.Errorf("can't ban user: %s. Error: %s", user.Username, err.Error())
				success = false
			}
		}

		if !keepBanned {
			if err := d.restrictor.Unban(ctx, channelToBan, user.ID); err != nil {
				d.log.Errorf("can't unban user: %s. Error: %s", user.Username, err.Error())
				success = false
			}
		}
		if success {
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
		keepBanned,
		cleanMessages,
		cleanUnknown,
	)

	for _, chatID := range report.ReportChannels {
		if err := d.restrictor.SendReportMessage(ctx, chatID, message); err != nil {
			d.log.Errorf("can't send message: %s. Message text: %s", err, message)
		}
	}
	d.log.Debugf(message)
}
