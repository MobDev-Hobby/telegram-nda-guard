package bot

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) PromoteUser(
	ctx context.Context,
	channelID int64,
	userID int64,
	options ...guard.Permission,
) (bool, error) {

	promotion := &bot.PromoteChatMemberParams{
		ChatID: channelID,
		UserID: userID,
	}

	for _, option := range options {
		switch option {
		case guard.CanInviteUsers:
			promotion.CanInviteUsers = true
		case guard.CanPromoteMembers:
			promotion.CanPromoteMembers = true
		case guard.CanRestrictMembers:
			promotion.CanRestrictMembers = true
		}
	}

	return d.GetBot().PromoteChatMember(ctx, promotion)
}

func (d *Domain) CheckAccessUser(
	ctx context.Context,
	channelID int64,
	userID int64,
	options ...guard.Permission,
) (bool, map[guard.Permission]bool, error) {

	var isMember bool
	access := make(map[guard.Permission]bool)
	for _, option := range options {
		access[option] = false
	}

	rights, err := d.GetBot().GetChatMember(
		ctx,
		&bot.GetChatMemberParams{
			ChatID: channelID,
			UserID: userID,
		},
	)

	if err != nil {
		d.log.Errorf("GetChatMember: %v", err)
		return false, access, err
	}

	switch rights.Type {
	case models.ChatMemberTypeOwner, models.ChatMemberTypeAdministrator, models.ChatMemberTypeMember:
		isMember = true
	case models.ChatMemberTypeRestricted:
		isMember = rights.Restricted != nil && rights.Restricted.IsMember
	case models.ChatMemberTypeLeft, models.ChatMemberTypeBanned:
		isMember = false
	}

	if rights.Administrator == nil {
		return isMember, access, nil
	}

	for _, option := range options {
		switch option {
		case guard.CanInviteUsers:
			access[option] = rights.Administrator.CanInviteUsers
		case guard.CanPromoteMembers:
			access[option] = rights.Administrator.CanPromoteMembers
		case guard.CanRestrictMembers:
			access[option] = rights.Administrator.CanRestrictMembers
		}
	}

	return isMember, access, nil
}
