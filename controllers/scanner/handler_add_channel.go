package scanner

import (
	"context"
	"fmt"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
)

func (d *Domain) AddChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("Chat add request got from chat: %d, user: %s", update.Message.ChatID, update.Message.User.Username)

	// Telegram's request_chat picker shows either groups or channels, never
	// both, so offer one button per kind. Each gets its own request id; both
	// resolve to the chat the /add came from.
	groupRequestID := d.nextAddChannelRequestID()
	channelRequestID := d.nextAddChannelRequestID()
	d.channelsMutex.Lock()
	d.addChannelHandlers[int(groupRequestID)] = update.Message.ChatID
	d.addChannelHandlers[int(channelRequestID)] = update.Message.ChatID
	d.channelsMutex.Unlock()

	requestChat := true
	err := d.telegramBot.SendMessage(
		ctx,
		&guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: "Choose what to protect. The bot will be added as an administrator " +
				"with the right to ban users.",
			Buttons: [][]guard.Button{
				{
					{
						Text:                 "Add channel",
						ID:                   channelRequestID,
						RequestChannel:       &requestChat,
						RequestChatIsChannel: true,
					},
					{
						Text:           "Add group",
						ID:             groupRequestID,
						RequestChannel: &requestChat,
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
	if update.Message.ChatType == guard.ChatTypePrivate {
		// Added from a private chat: the user administers the chat (the
		// picker guarantees it) and manages it from the Mini App.
		protectedChannel.Managers = []int64{update.Message.User.ID}
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

	if pc, ok := d.getProtectedChannel(update.Message.ChatShared.ChatID); ok {
		d.NoteUserName(update.Message.User.ID, strings.TrimSpace(update.Message.User.FirstName+" "+update.Message.User.LastName))
		d.recordAudit(ctx, pc, audit.Event{
			ActorID: update.Message.User.ID, Action: audit.ActionChannelAdded,
			Details: map[string]any{"controlChat": update.Message.ChatID},
		}, "")
	}

	d.log.Infof("Added protected channel: %d with admin chat: %d/%s", update.Message.ChatShared.ChatID, update.Message.ChatID, update.Message.User.Username)

	chanInfo, _ := d.getChannel(update.Message.ChatShared.ChatID)

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
				"%s <b>%s</b> added! \nCheck permissions:\n • Scan - %t\n • Clean - %t",
				capitalize(guard.ChatTypeNoun(chanInfo.chatType)),
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

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
