package scanner

import (
	"context"
	"fmt"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) Run(
	ctx context.Context,
) error {

	if d.storage != nil {
		if d.storage != nil &&
			(d.defaultAccessChecker == nil || d.defaultCleanProcessor == nil || d.defaultScanProcessor == nil) {
			return fmt.Errorf("you must set default access checker, default clean processor and default scan processor while using storage")
		}
		protectedChannels, err := d.storage.LoadAll(ctx)
		if err != nil {
			return fmt.Errorf("can't load protected channels list: %w", err)
		}
		for _, protectedChannel := range protectedChannels {
			protectedChannel := ProtectedChannel{
				ID:                protectedChannel.ID,
				CommandChannelIDs: protectedChannel.CommandChannelIDs,
				AutoScan:          protectedChannel.AutoScan,
				AutoClean:         protectedChannel.AutoClean,
				AllowClean:        protectedChannel.AllowClean,
			}
			protectedChannel.CleanReportProcessor = d.defaultCleanProcessor
			protectedChannel.ScanReportProcessor = d.defaultScanProcessor
			protectedChannel.AccessChecker = d.defaultAccessChecker
			d.AddDefaultProtectedChannel(&protectedChannel)
		}
	}

	d.log.Debugf("Run telegram bot")
	err := d.telegramBot.Run(ctx)
	if err != nil {
		return fmt.Errorf("can't run telegram bot: %w", err)
	}
	d.log.Infof("Telegram bot launched")

	d.log.Debugf("Run telegram user bot")
	d.runUserBot(ctx)
	d.log.Debugf("Telegram user bot launched")

	d.log.Debugf("Setup telegram bot handlers")
	d.setupCommands(ctx)
	d.log.Debugf("Telegram bot handlers registered")

	d.log.Debugf("Setup access levels for bot and userbots")
	err = d.CheckRights(ctx)
	if err != nil {
		d.log.Errorf("Can't check rights: %v", err)
	}

	d.log.Debugf("Setup access levels checker loop")
	d.RunAccessRightsChecker(ctx)

	d.log.Debugf("Setup user checker loop")
	d.RunUserAccessChecker(ctx)

	d.log.Infof("Initialization completed, now bot is ready to go")
	d.notifySuccessRun(ctx)

	return nil
}

func (d *Domain) notifySuccessRun(ctx context.Context) {
	d.NotifyAdmin(
		ctx,
		"Bot initialization completed",
	)

	if d.withCommonLaunchNotify {
		for commandChannelID, controlledChannels := range d.commandChannels {
			channels := make([]string, 0, len(controlledChannels))
			for _, channelID := range controlledChannels {
				protectedChannel, ok := d.protectedChannels[channelID]
				if !ok {
					continue
				}

				title := fmt.Sprintf("%d", channelID)
				channel, ok := d.channels[channelID]
				if ok {
					title = channel.title
				}
				channels = append(
					channels,
					fmt.Sprintf(
						"\n• <b>%s</b>\n \t • Auto scan: <b>%t</b>\n \t • Auto clean: <b>%t</b>\n \t • Manual clean: <b>%t</b>\n",
						title,
						protectedChannel.AutoClean,
						protectedChannel.AutoScan,
						protectedChannel.AllowClean,
					),
				)
			}

			_ = d.telegramBot.SendMessage(
				ctx, &guard.Message{
					ChatID: commandChannelID,
					Text: "Hello!\nFew channels linked to this control channel:\n" +
						strings.Join(channels, "\n") +
						"\n\nUse /help to see all commands",
					Buttons: d.getDefaultButtons(),
				},
			)
		}
	}
}

func (d *Domain) runUserBot(ctx context.Context) {
	d.log.Debugf("Start user bot initialization")
	err := d.userBot.Run(
		ctx,
	)
	d.log.Debugf("User bot initialization finished")
	if err != nil {
		d.log.Debugf("Userbot launch error, need retry")
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID: d.adminUserChatID,
				Text:   "Userbot launch error, enter /retry to retry",
			},
		)
		if err != nil {
			d.log.Errorf("Can't send message: %v", err)
		}
	}
}
