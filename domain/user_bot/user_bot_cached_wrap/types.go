package user_bot_cached_wrap

import (
	"time"

	"github.com/gotd/td/tg"
)

type Cache struct {
	channelUsersCache map[int64]ChannelUsersCache
}

type ChannelUsersCache struct {
	updated time.Time
	users   []tg.User
}
