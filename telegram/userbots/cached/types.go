package cached

import (
	"sync"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

type Cache struct {
	mutex             sync.Mutex
	channelUsersCache map[int64]ChannelUsersCache
}

type ChannelUsersCache struct {
	updated time.Time
	users   map[int64]guard.User
}
