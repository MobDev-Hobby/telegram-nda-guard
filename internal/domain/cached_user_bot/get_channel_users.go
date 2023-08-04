package cached_user_bot

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

func (wrap *CachedBotWrap) GetChannelUsers(
	ctx context.Context,
	channelId int64,
) ([]tg.User, error) {

	if cache, found := wrap.d.cache.channelUsersCache[channelId]; found {
		wrap.d.log.Debugf("cache found for channel %d", channelId)
		if cache.updated.Add(wrap.d.cacheTime).After(time.Now()) {
			wrap.d.log.Debugf("cache hit for channel %d", channelId)
			return cache.users, nil
		}
		wrap.d.log.Debugf("cache expired for channel %d", channelId)
	}
	wrap.d.log.Debugf("skip cache, get new data %d", channelId)

	users, err := wrap.bot.GetChannelUsers(ctx, channelId)
	if err != nil {
		return users, err
	}

	wrap.d.log.Debugf("update cache for channel %d", channelId)
	wrap.d.cache.channelUsersCache[channelId] = ChannelUsersCache{
		updated: time.Now(),
		users:   users,
	}
	return users, nil
}
