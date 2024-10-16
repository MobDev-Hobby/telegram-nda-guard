package userbot

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) GetChannelUsers(
	ctx context.Context,
	channelID int64,
) ([]guard.User, error) {

	var users = make([]guard.User, 0)

	chatList, err := d.userBot.client.API().ChannelsGetChannels(
		ctx,
		[]tg.InputChannelClass{
			(&tg.Channel{
				ID: d.userBot.botChannelIDtoUserChannelID(channelID),
			}).AsInput(),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("can't get channel: %w", err)
	}

	if len(chatList.GetChats()) == 0 {
		return nil, fmt.Errorf("can't get channel: nil")
	}

	for _, contact := range chatList.GetChats() {
		channel, ok := contact.(*tg.Channel)
		if channel != nil && ok {
			offset := 0
			page := 100
			for {
				resp, err := d.userBot.client.API().ChannelsGetParticipants(
					ctx, &tg.ChannelsGetParticipantsRequest{
						Channel: channel.AsInput(),
						Filter:  &tg.ChannelParticipantsRecent{},
						Offset:  offset,
						Limit:   page,
						Hash:    0,
					},
				)

				if err != nil {
					return nil, fmt.Errorf("error get participants: %w", err)
				}

				participants, ok := resp.(*tg.ChannelsChannelParticipants)
				if participants != nil && ok {
					for _, userObj := range participants.Users {
						user, ok := userObj.(*tg.User)
						if user != nil && ok {

							usernames := []string{}
							for _, username := range user.Usernames {
								usernames = append(usernames, username.Username)
							}

							users = append(
								users,
								guard.User{
									ID:        user.ID,
									Username:  user.Username,
									FirstName: user.FirstName,
									LastName:  user.LastName,
									Phone:     &user.Phone,
									Usernames: usernames,
								},
							)
						}
					}
				}

				if participants == nil || len(participants.Participants) < page {
					break
				}
				offset += page
			}
		}
	}

	return users, nil
}
