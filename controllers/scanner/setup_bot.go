package scanner

import (
	"context"
	"fmt"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) RunAccessRightsChecker(ctx context.Context) {
	ticker := time.NewTicker(d.accessCheckInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				err := d.CheckRights(ctx)
				if err != nil {
					d.log.Errorf("Access actualization failed with error %s", err)
					continue
				}
				d.log.Debugf("Access actualization done")
			}
		}
	}()
}
func (d *Domain) CheckRights(ctx context.Context) error {
	var err error

	err = d.updateChannelInfo(ctx)
	if err != nil {
		d.log.Errorf("Unable to update channel info: %s", err.Error())
		return err
	}

	err = d.updateBotAccessRights(ctx)
	if err != nil {
		d.log.Errorf("Unable to get access rights: %v", err)
		return err
	}

	return nil
}

func (d *Domain) updateChannelInfo(ctx context.Context) error {
	for channelID := range d.channels {
		channel, err := d.telegramBot.GetChat(ctx, channelID)

		if err != nil {
			d.log.Errorf("GetChat: %v", err)
		}

		if channel != nil {
			channelInfo := d.channels[channelID]
			channelInfo.title = channel.Title
			d.channels[channelID] = channelInfo

			if channel.MigratedTo != nil {
				err := d.MigrateChannel(channelID, *channel.MigratedTo)
				if err != nil {
					d.log.Errorf("Migrate error: %v", err)
					continue
				}
				d.NotifyAdmin(
					ctx,
					fmt.Sprintf(
						"Telegram migrated channel %s.\n\n"+
							"Old ID: <b>%d</b>\n"+
							"New ID: <b>%d</b>\n\n"+
							"Now I handled it, but you should update config.",
						d.channels[channelID].title,
						channelID,
						*channel.MigratedTo,
					),
				)
			}
		}
	}

	return nil
}

func (d *Domain) updateBotAccessRights(ctx context.Context) error {
	for channelID, channel := range d.channels {
		protectedChannel, ok := d.protectedChannels[channelID]
		if !ok {
			d.log.Panicf("Channel %s not found in protected channels", channel.title)
			return nil
		}

		isMember, permissions, err := d.telegramBot.CheckAccessUser(
			ctx,
			channelID,
			d.telegramBot.UserID(),
			guard.CanInviteUsers,
			guard.CanRestrictMembers,
		)

		if err != nil {
			d.log.Errorf("Gen't check access rights: %v", err)
			continue
		}

		if !isMember {
			d.NotifyAdmin(
				ctx,
				fmt.Sprintf(
					"I need to be member of channel [%d]<b>%s</b>, add me, please",
					channelID,
					channel.title,
				),
			)
			channel.botOnChannel = false
			d.channels[channelID] = channel
			continue
		}

		channel.botOnChannel = true

		channel.botCanInvite = permissions[guard.CanInviteUsers]
		channel.botCanClean = permissions[guard.CanRestrictMembers]

		if !permissions[guard.CanRestrictMembers] &&
			(protectedChannel.AllowClean || protectedChannel.AutoClean) {

			d.NotifyAdmin(
				ctx,
				fmt.Sprintf(
					"I need to have admin permissins to clean channel [%d]<b>%s</b>, promote me, please",
					channelID,
					channel.title,
				),
			)
		}

		d.channels[channelID] = channel
	}

	return nil
}
