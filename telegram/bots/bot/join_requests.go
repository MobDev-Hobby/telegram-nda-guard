package bot

import (
	"context"

	"github.com/go-telegram/bot"
)

// ApproveJoinRequest lets userID into chatID. The bot needs the
// can_invite_users right.
func (d *Domain) ApproveJoinRequest(ctx context.Context, chatID, userID int64) error {
	_, err := d.botClient.ApproveChatJoinRequest(ctx, &bot.ApproveChatJoinRequestParams{
		ChatID: normalizeBotChatID(chatID),
		UserID: userID,
	})
	return err
}

// DeclineJoinRequest turns userID's request to join chatID down.
func (d *Domain) DeclineJoinRequest(ctx context.Context, chatID, userID int64) error {
	_, err := d.botClient.DeclineChatJoinRequest(ctx, &bot.DeclineChatJoinRequestParams{
		ChatID: normalizeBotChatID(chatID),
		UserID: userID,
	})
	return err
}
