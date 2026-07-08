package bot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// GetChatAdministrators returns the user IDs of the owner and administrators of
// chatID. It maps the Bot API GetChatAdministrators response (a discriminated
// union of ChatMember sub-types) to a flat list of user IDs suitable for
// membership checks (e.g. by the default authorizer).
func (d *Domain) GetChatAdministrators(ctx context.Context, chatID int64) ([]int64, error) {
	members, err := d.botClient.GetChatAdministrators(
		ctx,
		&bot.GetChatAdministratorsParams{
			ChatID: chatID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get chat administrators: %w", err)
	}

	ids := make([]int64, 0, len(members))
	for _, m := range members {
		if id := chatMemberUserID(m); id != 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// chatMemberUserID extracts the user ID from any ChatMember variant. The Bot API
// models.ChatMember is a discriminated union; each variant carries the user in a
// different sub-struct. Returns 0 when the variant or its user is absent.
func chatMemberUserID(m models.ChatMember) int64 {
	switch {
	case m.Owner != nil && m.Owner.User != nil:
		return m.Owner.User.ID
	case m.Administrator != nil:
		return m.Administrator.User.ID
	case m.Member != nil && m.Member.User != nil:
		return m.Member.User.ID
	case m.Restricted != nil && m.Restricted.User != nil:
		return m.Restricted.User.ID
	case m.Left != nil && m.Left.User != nil:
		return m.Left.User.ID
	case m.Banned != nil && m.Banned.User != nil:
		return m.Banned.User.ID
	}
	return 0
}
