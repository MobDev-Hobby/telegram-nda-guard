package scanner

import (
	"context"
	"fmt"
)

func (d *Domain) inviteUserBotToChat(ctx context.Context, channelID int64) error {
	link, err := d.telegramBot.GetInviteLink(ctx, channelID)
	if err != nil {
		return fmt.Errorf("CreateChatInviteLink failed: %w", err)
	}

	d.log.Debugf("CreateChatInviteLink result: %+v", link)

	err = d.userBot.JoinChannelByInviteLink(ctx, link)
	if err != nil {
		return fmt.Errorf("JoinChatInviteLink failed: %w", err)
	}

	return nil
}

func (d *Domain) updateUserBotAccessRights(ctx context.Context) error {
	for channelID, channel := range d.channels {
		isMember, _, err := d.telegramBot.CheckAccessUser(
			ctx,
			channelID,
			d.userBot.UserID(),
		)

		if err != nil {
			d.log.Errorf("Can't check userbot permissions: %v", err)
			continue
		}

		if !isMember {
			d.log.Debugf("User bot is not in channel [%d]%s yet, try to add...", channelID, channel.title)
			if !channel.botCanInvite {
				d.NotifyAdmin(
					ctx,
					fmt.Sprintf(
						"I need admin rights to invite and promote users in channel <b>%s</b>, "+
							"promote me, please, or invite my buddy %s to the channel",
						channel.title,
						d.userBot.Username(),
					),
				)
				channel.userBotOnChannel = false
				d.channels[channelID] = channel
				continue
			}
			err := d.inviteUserBotToChat(ctx, channelID)
			if err != nil {
				d.log.Errorf("inviteUserBotToChat: %v", err)
				d.NotifyAdmin(
					ctx,
					fmt.Sprintf(
						"Can't invite userbot <b>%s</b> to the channel <b>%s</b>\n"+
							"Is user has access to the channel?\n\n"+
							"Check it, please, I'm not so smart to do it myself yet.",
						d.userBot.Username(),
						channel.title,
					),
				)
				channel.userBotOnChannel = false
				d.channels[channelID] = channel
				continue
			}

			d.log.Debugf("User bot joined channel [%d]%s", channelID, channel.title)
		}

		channel.userBotOnChannel = true
		d.channels[channelID] = channel
	}

	return nil
}
