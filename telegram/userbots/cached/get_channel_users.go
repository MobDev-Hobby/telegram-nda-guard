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

	// The cache map is also written by the MTProto update handlers, so every
	// access goes through the mutex.
	d.cache.mutex.Lock()
	cache, found := d.cache.channelUsersCache[channelID]
	if found && cache.updated.Add(d.cacheTime).After(time.Now()) {
		users := make([]guard.User, 0, len(cache.users))
		for _, user := range cache.users {
			users = append(users, user)
		}
		d.cache.mutex.Unlock()
		d.log.Debugf("cache hit for channel %d", channelID)
		return users, nil
	}
	d.cache.mutex.Unlock()
	d.log.Debugf("skip cache, get new data %d", channelID)

	users, err := d.userBot.GetChannelUsers(ctx, channelID)
	if err != nil {
		return users, err
	}

	cacheUsers := make(map[int64]guard.User)
	for _, user := range users {
		cacheUsers[user.ID] = user
	}

	d.cache.mutex.Lock()
	d.cache.channelUsersCache[channelID] = ChannelUsersCache{
		updated: time.Now(),
		users:   cacheUsers,
	}
	d.cache.mutex.Unlock()
	return users, nil
}

// InvalidateChannel drops the cached member list of channelID, e.g. after
// users were kicked, so the next scan reflects the change.
func (d *Domain) InvalidateChannel(channelID int64) {
	d.cache.mutex.Lock()
	delete(d.cache.channelUsersCache, channelID)
	d.cache.mutex.Unlock()
}

// ChannelStats forwards the completeness of the last member listing when the
// wrapped userbot tracks it.
func (d *Domain) ChannelStats(channelID int64) (guard.ScanStats, bool) {
	provider, ok := d.userBot.(interface {
		ChannelStats(channelID int64) (guard.ScanStats, bool)
	})
	if !ok {
		return guard.ScanStats{}, false
	}
	return provider.ChannelStats(channelID)
}
