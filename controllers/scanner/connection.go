package scanner

import (
	"context"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// Publish the protected channel before Telegram round trips. Never run the
// entire rights sweep in the HTTP connect request: large installations can
// take minutes and leave users uncertain whether the operation succeeded.
func (d *Domain) refreshConnectedChat(channelID int64) <-chan struct{} {
	d.channelsMutex.Lock()
	ch := d.channels[channelID]
	if ch.connecting && ch.connectDone != nil {
		d.channelsMutex.Unlock()
		return ch.connectDone
	}
	done := make(chan struct{})
	ch.connecting, ch.rightsChecked, ch.connectDone = true, false, done
	d.channels[channelID] = ch
	d.channelsMutex.Unlock()
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		chat, chatErr := d.telegramBot.GetChat(ctx, channelID)
		member, permissions, err := d.telegramBot.CheckAccessUser(ctx, channelID, d.telegramBot.UserID(), guard.CanInviteUsers, guard.CanRestrictMembers)
		if chatErr != nil {
			d.log.Warnf("chat %d identity refresh: %s", channelID, chatErr)
		}
		if err != nil {
			d.log.Warnf("chat %d rights refresh: %s", channelID, err)
		}
		if chat != nil {
			if e := d.updateProtectedChannel(ctx, channelID, func(pc *ProtectedChannel) {
				if chat.Title != "" {
					pc.Title = chat.Title
				}
				pc.ChatType = chat.Type
			}); e != nil {
				d.log.Warnf("chat %d identity persistence: %s", channelID, e)
			}
		}
		d.channelsMutex.Lock()
		defer d.channelsMutex.Unlock()
		if _, exists := d.protectedChannels[channelID]; !exists {
			return
		}
		current := d.channels[channelID]
		current.connecting = false
		if chat != nil {
			if chat.Title != "" {
				current.title = chat.Title
			}
			current.chatType = chat.Type
		}
		current.rightsChecked = err == nil
		if err == nil {
			current.botOnChannel = member
			current.botCanClean = permissions[guard.CanRestrictMembers]
			current.botCanInvite = permissions[guard.CanInviteUsers]
		}
		d.channels[channelID] = current
	}()
	return done
}
