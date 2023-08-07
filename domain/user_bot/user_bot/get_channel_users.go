package user_bot

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

func (bot UserBotInstance) GetChannelUsers(
	ctx context.Context,
	channelId int64,
) ([]tg.User, error) {

	var users = make([]tg.User, 0)
	chatList, err := bot.client.API().ChannelsGetChannels(
		ctx,
		[]tg.InputChannelClass{
			(&tg.Channel{
				ID: channelId,
			}).AsInput(),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("can't get channel: %w", err)
	}

	for _, contact := range chatList.GetChats() {
		channel := contact.(*tg.Channel)
		if channel != nil {
			offset := 0
			page := 100
			for {
				resp, err := bot.client.API().ChannelsGetParticipants(
					ctx, &tg.ChannelsGetParticipantsRequest{
						Channel: channel.AsInput(),
						Filter:  &tg.ChannelParticipantsRecent{},
						Offset:  offset,
						Limit:   page,
						Hash:    0,
					},
				)

				if err != nil {
					return nil, fmt.Errorf("error get participants: %s", err)
				}

				participants := resp.(*tg.ChannelsChannelParticipants)
				if participants != nil {
					for _, userObj := range participants.Users {
						user := userObj.(*tg.User)
						if user != nil {
							users = append(
								users,
								*user,
							)
						}
					}
				}

				if len(participants.Participants) < page {
					break
				}
				offset += page
			}
		}

	}

	return users, nil
}
