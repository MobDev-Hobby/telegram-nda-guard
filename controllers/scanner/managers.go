package scanner

import (
	"context"
	"fmt"
	"slices"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
)

// IsChannelManager implements MiniAppService: whether userID joined the
// channel in the bot.
func (d *Domain) IsChannelManager(channelID, userID int64) bool {
	pc, ok := d.getProtectedChannel(channelID)
	return ok && slices.Contains(pc.Managers, userID)
}

// JoinChannel implements MiniAppService: a channel administrator becomes a
// manager of the channel in the bot. The caller's administrator status is
// checked by the transport before calling this.
func (d *Domain) JoinChannel(ctx context.Context, channelID, callerID int64) error {
	joined := false
	err := d.updateProtectedChannel(ctx, channelID, func(pc *ProtectedChannel) {
		if slices.Contains(pc.Managers, callerID) {
			return
		}
		pc.Managers = append(slices.Clone(pc.Managers), callerID)
		joined = true
	})
	if err != nil || !joined {
		return err
	}
	pc, _ := d.getProtectedChannel(channelID)
	d.recordAudit(ctx, pc, audit.Event{ActorID: callerID, Action: audit.ActionChannelJoined},
		fmt.Sprintf("%s joined %s as a manager.", actorLink(callerID, d.userName(callerID)), d.channelTitle(channelID)))
	return nil
}

// notifyManagers sends text to every manager in private. Managers who never
// started the bot can't be messaged; that is logged and skipped.
func (d *Domain) notifyManagers(ctx context.Context, pc ProtectedChannel, text string) {
	for _, userID := range pc.Managers {
		if err := d.telegramBot.SendMessage(ctx, d.withAppButton(pc.ID, userID, text)); err != nil {
			d.log.Warnf("can't message manager %d of %d: %s", userID, pc.ID, err)
		}
	}
}
