package cached

import (
	"context"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) GetChannelUsers(
	ctx context.Context,
	channelID int64,
) ([]guard.User, error) {

	if cache, found := d.cache.channelUsersCache[channelID]; found {
		d.log.Debugf("cache found for channel %d", channelID)
		if cache.updated.Add(d.cacheTime).After(time.Now()) {
			d.log.Debugf("cache hit for channel %d", channelID)
			users := make([]guard.User, 0, len(cache.users))
			for _, user := range cache.users {
				users = append(users, user)
			}
			return users, nil
		}
		d.log.Debugf("cache expired for channel %d", channelID)
	}
	d.log.Debugf("skip cache, get new data %d", channelID)

	users, err := d.userBot.GetChannelUsers(ctx, channelID)
	if err != nil {
		return users, err
	}

	cacheUsers := make(map[int64]guard.User)
	for _, user := range users {
		cacheUsers[user.ID] = user
	}

	d.log.Debugf("update cache for channel %d", channelID)
	d.cache.channelUsersCache[channelID] = ChannelUsersCache{
		updated: time.Now(),
		users:   cacheUsers,
	}
	return users, nil
}
