package scanner

import (
	"context"
	"fmt"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) AddChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("Chat add request got from chat: %d", update.Message.ChatID, update.Message.User.Username)

	requestId := int32(update.Message.ChatID*1000 + time.Now().UnixNano())
	d.addChannelHandlers[int(requestId)] = update.Message.ChatID

	err := d.telegramBot.SendAddChannelButton(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "Press button to add protected channel",
		},
		int32(requestId),
		"Select channel",
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)
}

func (d *Domain) AddChannelCallbackHandler(
	ctx context.Context,
	update *guard.Update,
) {

	if update.Message == nil || update.Message.ChatShared == nil {

		d.log.Infof("Unexpected chat add request got, nil Message")
		return
	}

	requestID := update.Message.ChatShared.RequestID
	chatId, expected := d.addChannelHandlers[requestID]
	if !expected || chatId != update.Message.ChatID {

		d.log.Infof("Unexpected chat add request got, requestID: %d, chatId: %c", requestID, chatId)

		err := d.telegramBot.SendMessage(
			ctx,
			&guard.Message{
				ChatID:   update.Message.ChatID,
				ThreadID: update.Message.ThreadID,
				Text:     "Unexpected channel, use /add please",
			},
		)

		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}

		return
	}

	delete(d.addChannelHandlers, requestID)

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)

	err := d.AddProtectedChannel(&ProtectedChannel{
		ID:                   update.Message.ChatShared.ChatID,
		AutoScan:             true,
		CommandChannelIDs:    []int64{update.Message.ChatID},
		ScanReportProcessor:  d.defaultScanProcessor,
		CleanReportProcessor: d.defaultCleanProcessor,
		AccessChecker:        d.defaultAccessChecker,
	})

	if err != nil {
		d.log.Errorf("can't add protected channel: %s", err)
		return
	}

	err = d.CheckRights(ctx)
	if err != nil {
		d.log.Errorf("can't check rights: %v", err)
	}

	d.log.Infof("Added protected channel: %d with admin chat: %d/%s", update.Message.ChatShared.ChatID, update.Message.ChatID, update.Message.User.Username)

	chanInfo := d.channels[update.Message.ChatShared.ChatID]

	err = d.telegramBot.SendMessage(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: fmt.Sprintf(
				"Channel <b>%s</b> added! \nCheck permissions:\n • /scan - %t\n • /clean - %t",
				chanInfo.title,
				chanInfo.CanScan(),
				chanInfo.CanClean(),
			),
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}
}
