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

	opts := d.effectiveOptions(report.CleanOptions)

	usersToClean := report.DeniedUsers
	if opts.CleanUnknown {
		usersToClean = append(usersToClean, report.UnknownUsers...)
	}

	cleanedUsers := 0
	for _, result := range d.KickUsers(ctx, report.Channel, usersToClean, &opts) {
		if result.OK {
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
		opts.KeepBanned,
		opts.CleanMessages,
		opts.CleanUnknown,
	)
	if report.Stats.Partial() {
		message += fmt.Sprintf(
			"\n\n⚠️ Telegram returned only <b>%d of %d</b> members, the rest were not checked.",
			report.Stats.Fetched,
			report.Stats.Total,
		)
	}

	for _, chatID := range report.ReportChannels {
		if err := d.botClient.SendReport(ctx, chatID, message); err != nil {
			d.log.Errorf("can't send message: %s. Message text: %s", err, message)
		}
	}
	d.log.Debugf(message)
}

// KickUsers removes users from channel one by one and reports the outcome for
// each. opts overrides the kicker defaults when not nil. Unlike ProcessReport it
// sends no report, so callers that pick users by hand (e.g. the Mini App) can
// word their own.
func (d *Domain) KickUsers(
	ctx context.Context,
	channel guard.ChannelInfo,
	users []guard.User,
	opts *processors.CleanOptions,
) []processors.KickResult {

	effective := d.effectiveOptions(opts)
	results := make([]processors.KickResult, 0, len(users))
	for _, user := range users {
		err := d.cleanUser(ctx, channel, user, effective)
		result := processors.KickResult{UserID: user.ID, OK: err == nil}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

// effectiveOptions resolves per-channel overrides against the kicker defaults.
func (d *Domain) effectiveOptions(override *processors.CleanOptions) processors.CleanOptions {
	if override != nil {
		return *override
	}
	return processors.CleanOptions{
		KeepBanned:    d.keepBanned,
		CleanMessages: d.cleanMessages,
		CleanUnknown:  d.cleanUnknown,
	}
}

// DefaultCleanOptions returns the process-wide defaults, used by channels
// without their own CleanOptions.
func (d *Domain) DefaultCleanOptions() processors.CleanOptions {
	return d.effectiveOptions(nil)
}

// cleanUser bans then (optionally) unbans a single user. The bot client is
// expected to be rate-limited and to handle Telegram FLOOD_WAIT (429)
// internally.
func (d *Domain) cleanUser(
	ctx context.Context,
	channel guard.ChannelInfo,
	user guard.User,
	opts processors.CleanOptions,
) error {

	// Removing a member always takes a ban: Telegram has no plain "kick".
	// Without KeepBanned the ban is lifted right away, so the user can rejoin
	// through an invite link later.
	if err := d.botClient.Ban(ctx, channel.ID, user.ID, opts.CleanMessages); err != nil {
		d.log.Errorf("can't ban user %d (%s): %s", user.ID, user.Username, err)
		return fmt.Errorf("ban: %w", err)
	}

	if !opts.KeepBanned {
		if err := d.botClient.Unban(ctx, channel.ID, user.ID); err != nil {
			d.log.Errorf("can't unban user %d (%s): %s", user.ID, user.Username, err)
			return fmt.Errorf("unban: %w", err)
		}
	}

	return nil
}
