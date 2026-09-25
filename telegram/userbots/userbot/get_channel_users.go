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
	total := 0

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
					if participants.Count > total {
						total = participants.Count
					}
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

	stats := guard.ScanStats{Fetched: len(users), Total: total}
	if stats.Total < stats.Fetched {
		stats.Total = stats.Fetched
	}
	if stats.Partial() {
		// Telegram hides part of the member list of large broadcast channels
		// even from admins. Record it so reports can say the list is partial.
		d.log.Warnf("channel %d: fetched %d of %d members", channelID, stats.Fetched, stats.Total)
	}
	d.statsMutex.Lock()
	d.stats[channelID] = stats
	d.statsMutex.Unlock()

	return users, nil
}

// ChannelStats returns how complete the last member listing of channelID was.
func (d *Domain) ChannelStats(channelID int64) (guard.ScanStats, bool) {
	d.statsMutex.Lock()
	defer d.statsMutex.Unlock()
	stats, ok := d.stats[channelID]
	return stats, ok
}
