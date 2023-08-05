package cached_user_bot

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
