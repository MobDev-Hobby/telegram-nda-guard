package cached

import (
	"context"

	"github.com/gotd/td/tg"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) HandleNewChannelMessage(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateNewChannelMessage,
) error {

	message, ok := update.Message.(*tg.MessageService)
	if !ok {
		return nil
	}

	var chatID int64
	switch peer := message.PeerID.(type) {
	case *tg.PeerChannel:
		chatID = d.userChannelIDtoBotChannelID(peer.ChannelID)
	case *tg.PeerChat:
		chatID = d.userChannelIDtoBotChannelID(peer.ChatID)
	default:
		return nil
	}

	switch action := message.Action.(type) {
	case *tg.MessageActionChatAddUser:
		for _, userID := range action.Users {
			user, ok := e.Users[userID]
			if !ok {
				continue
			}
			d.addUserToCache(ctx, user, chatID)
		}
	case *tg.MessageActionChatDeleteUser:
		d.delUserFromCache(ctx, action.UserID, chatID)
	}
	return nil
}

func (d *Domain) HandleChatParticipantAdd(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateChatParticipantAdd,
) error {

	if update == nil {
		return nil
	}
	chatID := d.userChannelIDtoBotChannelID(update.ChatID)

	user := e.Users[update.UserID]
	if user == nil {
		return nil
	}

	d.addUserToCache(ctx, user, chatID)

	return nil
}

func (d *Domain) HandleChatParticipantDelete(
	ctx context.Context,
	e tg.Entities,
	update *tg.UpdateChatParticipantDelete,
) error {

	if update == nil {
		return nil
	}

	chatID := d.userChannelIDtoBotChannelID(update.ChatID)

	user := e.Users[update.UserID]
	if user == nil {
		return nil
	}

	d.delUserFromCache(ctx, user.ID, chatID)

	return nil
}

func (d *Domain) addUserToCache(_ context.Context, user *tg.User, chatID int64) {
	d.cache.mutex.Lock()
	defer d.cache.mutex.Unlock()

	if cacheData, ok := d.cache.channelUsersCache[chatID]; ok {
		// We already have newer stub
		cacheData.users[user.ID] = guard.User{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.Username,
			Phone:     &user.Phone,
		}
		d.cache.channelUsersCache[chatID] = cacheData
	}
}

func (d *Domain) delUserFromCache(_ context.Context, userID int64, chatID int64) {
	d.cache.mutex.Lock()
	defer d.cache.mutex.Unlock()

	if cacheData, ok := d.cache.channelUsersCache[chatID]; ok {
		delete(cacheData.users, userID)
		d.cache.channelUsersCache[chatID] = cacheData
	}
}
