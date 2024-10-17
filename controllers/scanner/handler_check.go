package scanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) checkChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelByControlChatHandler(ctx, update, Scan)
}

func (d *Domain) cleanChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelByControlChatHandler(ctx, update, Clean)
}

func (d *Domain) processScanCallbackHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelByControlChatHandler(ctx, update, Scan)
}

func (d *Domain) processCleanCallbackHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.scanChannelByControlChatHandler(ctx, update, Clean)
}

func (d *Domain) scanChannelByControlChatHandler(
	ctx context.Context,
	update *guard.Update,
	requestType ScanRequestType,
) {

	switch {
	case update.Message != nil:

		commandChatID := update.Message.ChatID
		response := ""

		if len(d.commandChannels[commandChatID]) == 0 {
			response = fmt.Sprintf(
				"No connected chats found for this channel %d",
				commandChatID,
			)
		} else {
			response = d.scanChannelHandler(ctx, commandChatID, d.commandChannels[commandChatID], requestType)
		}

		if response != "" {
			err := d.telegramBot.SendMessage(
				ctx, &guard.Message{
					ChatID: update.Message.ChatID,
					Text:   response,
				},
			)
			if err != nil {
				d.log.Errorf("can't send message: %s", err)
				return
			}
		}

	case update.CallbackQuery != nil && update.CallbackQuery.Message != nil:
		commandChatID := update.CallbackQuery.Message.ChatID

		payload := strings.Split(update.CallbackQuery.Data, " ")
		if len(payload) < 2 {
			d.log.Errorf("can't parse payload: %s", update.CallbackQuery.Data)
			return
		}

		channelRequired, err := strconv.Atoi(payload[1])
		if err != nil {
			d.log.Errorf("can't parse payload: %s", update.CallbackQuery.Data)
			return
		}

		response := d.scanChannelHandler(ctx, commandChatID, []int64{int64(channelRequired)}, requestType)
		if response != "" {
			d.telegramBot.CallbackResponse(
				ctx,
				guard.CallbackResponse{
					ID:        update.CallbackQuery.ID,
					Text:      response,
					ShowAlert: true,
				})
		}
	}
}

func (d *Domain) scanChannelHandler(
	ctx context.Context,
	commandChannelID int64,
	chatsToScan []int64,
	requestType ScanRequestType,
) string {

	d.log.Debugf("process check for command chat: %d, %s", commandChannelID)

	channelsPlanned := make([]string, 0, len(chatsToScan))
	operation := "Operation"
	for _, channelID := range chatsToScan {
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
			operation = "Scan operation"
			reportProcessor = protectedChannel.ScanReportProcessor
		case Clean:
			operation = "Clean operation"
			reportProcessor = protectedChannel.CleanReportProcessor
		case AutoScan, AutoClean, None:
		default:
		}
		if reportProcessor == nil {
			continue
		}

		channelsPlanned = append(channelsPlanned, fmt.Sprintf("\n• %s", channel.title))
		d.processRequestChan <- ScanRequest{
			requestType:     requestType,
			channelInfo:     channel,
			accessChecker:   protectedChannel.AccessChecker,
			reportProcessor: reportProcessor,
			reportChannels:  &[]int64{commandChannelID},
		}
	}

	d.log.Debugf("Planned scans for channels: %s", strings.Join(channelsPlanned, ","))

	return fmt.Sprintf(
		"%s planned for channels:\n %s",
		operation,
		strings.Join(channelsPlanned, ","),
	)
}
