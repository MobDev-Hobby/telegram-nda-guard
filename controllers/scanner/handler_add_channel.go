package scanner

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) AddChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("Chat add request got from chat: %d, user: %s", update.Message.ChatID, update.Message.User.Username)

	// Use a 32-bit request id for the Telegram request_chat button. The
	// previous int32 cast of (ChatID*1000 + Nanos) overflowed for any
	// realistic chat id, causing collisions in the pending-request map.
	requestId := d.nextAddChannelRequestID()
	d.channelsMutex.Lock()
	d.addChannelHandlers[int(requestId)] = update.Message.ChatID
	d.channelsMutex.Unlock()

	requestChannel := true
	err := d.telegramBot.SendMessage(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "Press button to add protected channel",
			Buttons: [][]guard.Button{
				{
					{
						Text:           "Select channel",
						ID:             requestId,
						RequestChannel: &requestChannel,
					},
				},
			},
		},
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
	d.channelsMutex.RLock()
	chatId, expected := d.addChannelHandlers[requestID]
	d.channelsMutex.RUnlock()
	if !expected || chatId != update.Message.ChatID {

		d.log.Infof("Unexpected chat add request got, requestID: %d, chatId: %d", requestID, chatId)

		err := d.telegramBot.SendMessage(
			ctx,
			&guard.Message{
				ChatID:   update.Message.ChatID,
				ThreadID: update.Message.ThreadID,
				Text:     "Unexpected channel, use /add please",
				Buttons:  d.getDefaultButtons(),
			},
		)

		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}

		return
	}

	d.channelsMutex.Lock()
	delete(d.addChannelHandlers, requestID)
	d.channelsMutex.Unlock()

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)

	protectedChannel := &ProtectedChannel{
		ID:                update.Message.ChatShared.ChatID,
		CommandChannelIDs: []int64{update.Message.ChatID},
		AutoScan:          true,
		AllowClean:        true,
	}
	err := d.AddDefaultProtectedChannel(
		protectedChannel,
	)

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

	var buttons []guard.InlineButton

	if chanInfo.CanScan() {
		buttons = append(
			buttons,
			guard.InlineButton{
				Text:    "/scan",
				Command: fmt.Sprintf("/scan %d", chanInfo.id),
			},
		)
	}
	if chanInfo.CanClean() && protectedChannel.AllowClean {
		buttons = append(
			buttons,
			guard.InlineButton{
				Text:    "/clean",
				Command: fmt.Sprintf("/clean %d", chanInfo.id),
			},
		)
	}

	err = d.telegramBot.SendMessage(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "Wait...",
			Buttons:  d.getDefaultButtons(),
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
	}

	err = d.telegramBot.SendMessage(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: fmt.Sprintf(
				"Channel <b>%s</b> added! \nCheck permissions:\n • Scan - %t\n • Clean - %t",
				chanInfo.title,
				chanInfo.CanScan(),
				chanInfo.CanClean() && protectedChannel.AllowClean,
			),
			InlineButtons: [][]guard.InlineButton{buttons},
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}
}
