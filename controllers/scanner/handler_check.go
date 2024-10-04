package scanner

import (
	"context"
	"fmt"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) checkChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelHandler(ctx, update, Scan)
}

func (d *Domain) cleanChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelHandler(ctx, update, Clean)
}

func (d *Domain) scanChannelHandler(
	ctx context.Context,
	update *guard.Update,
	requestType ScanRequestType,
) {

	d.log.Debugf("process check for command chat: %d, %s", update.Message.ChatID)

	if len(d.commandChannels[update.Message.ChatID]) == 0 {
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   update.Message.ChatID,
				ThreadID: update.Message.ThreadID,
				Text: fmt.Sprintf(
					"No connected chats found for this channel %d",
					update.Message.ChatID,
				),
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}
		return
	}

	channelsPlanned := make([]string, 0, len(d.commandChannels[update.Message.ChatID]))
	for _, channelID := range d.commandChannels[update.Message.ChatID] {
		channel, ok := d.channels[channelID]
		if !ok {
			continue
		}
		protectedChannel, ok := d.protectedChannels[channel.id]
		if !ok {
			continue
		}
		var reportProcessor UserReportProcessor
		switch requestType {
		case Scan:
			reportProcessor = protectedChannel.ScanReportProcessor
		case Clean:
			reportProcessor = protectedChannel.CleanReportProcessor
		case AutoScan, AutoClean, None:
		default:
		}
		if reportProcessor == nil {
			continue
		}

		channelsPlanned = append(channelsPlanned, fmt.Sprintf("\n• <b>%s</b>", channel.title))
		d.processRequestChan <- ScanRequest{
			requestType:     requestType,
			channelInfo:     channel,
			accessChecker:   protectedChannel.AccessChecker,
			reportProcessor: reportProcessor,
		}
	}

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: fmt.Sprintf(
				"Operation planned for channels:\n %s",
				strings.Join(channelsPlanned, ","),
			),
		},
	)
	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	d.log.Debugf("Planned scans for channels: %s", strings.Join(channelsPlanned, ","))
}
