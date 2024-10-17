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
						ID:             int32(requestId),
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
	chatId, expected := d.addChannelHandlers[requestID]
	if !expected || chatId != update.Message.ChatID {

		d.log.Infof("Unexpected chat add request got, requestID: %d, chatId: %c", requestID, chatId)

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

	delete(d.addChannelHandlers, requestID)

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
