package bot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
)

// normalizeBotChatID converts a bot-API-style channel identifier into the
// signed "-100..." form expected by the Bot API for supergroups and channels.
//
// Telegram Bot API uses negative chat IDs of the form -100<channel_id> for
// supergroups and broadcast channels. Internally the framework carries the
// channel id either already in that signed form (negative) or as the raw
// positive channel id; this helper makes the two interchangeable at the
// transport boundary so callers can pass whichever form they hold.
func normalizeBotChatID(channelID int64) int64 {
	if channelID > 0 {
		normalized, _ := strconv.ParseInt(fmt.Sprintf("-100%d", channelID), 10, 64)
		return normalized
	}
	return channelID
}

// SendReportMessage sends an HTML plain-text message to chatID. It is the
// UserRestrictor counterpart to SendMessage, for processors (e.g. the kicker)
// that build their own message text and do not need inline/reply keyboards.
func (d *Domain) SendReportMessage(ctx context.Context, chatID int64, text string) error {
	_, err := d.botClient.SendMessage(
		ctx,
		&bot.SendMessageParams{
			ChatID:    chatID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
			LinkPreviewOptions: &models.LinkPreviewOptions{
				IsDisabled: utils.Ptr(true),
			},
		},
	)
	return err
}

// Ban restricts userID in channelID. When revokeMessages is true, the user's
// recent messages are deleted as part of the restriction.
func (d *Domain) Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error {
	_, err := d.botClient.BanChatMember(
		ctx,
		&bot.BanChatMemberParams{
			ChatID:         normalizeBotChatID(channelID),
			UserID:         userID,
			RevokeMessages: revokeMessages,
		},
	)
	return err
}

// Unban lifts a previous restriction on userID in channelID.
func (d *Domain) Unban(ctx context.Context, channelID, userID int64) error {
	_, err := d.botClient.UnbanChatMember(
		ctx,
		&bot.UnbanChatMemberParams{
			ChatID: normalizeBotChatID(channelID),
			UserID: userID,
			// OnlyIfBanned:true makes the unban a no-op (instead of an error)
			// when the preceding ban did not take effect, which is the only
			// case the kicker reaches this call after a failed ban.
			OnlyIfBanned: true,
		},
	)
	return err
}
