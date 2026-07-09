package scanner

import "sync/atomic"

// Thread-safe accessors for the channel maps guarded by channelsMutex.
//
// The four maps (commandChannels, protectedChannels, channels,
// addChannelHandlers) are read/written concurrently from:
//   - RunAccessRightsChecker goroutine (setup_bot.go)
//   - scan worker pool (channel_users_checker.go CheckChannelsLoop)
//   - the ticker goroutine (channel_users_checker.go RunUserAccessChecker)
//   - per-update Telegram handlers (handler_check.go, handler_list.go,
//     handler_add_channel.go, add_protected_channel.go)
//
// Every access must go through these helpers (or hold channelsMutex
// directly) to avoid fatal "concurrent map read and map write" panics.

// nextAddChannelRequestID returns a unique 32-bit id for a request_chat
// button. It replaces the overflow-prone int32(ChatID*1000+Nanos) that
// caused collisions in the pending-request map.
func (d *Domain) nextAddChannelRequestID() int32 {
	return atomic.AddInt32(&d.addChannelRequestCounter, 1)
}

func (d *Domain) getChannel(channelID int64) (ChannelInfo, bool) {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	ch, ok := d.channels[channelID]
	return ch, ok
}

func (d *Domain) getProtectedChannel(channelID int64) (ProtectedChannel, bool) {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	pc, ok := d.protectedChannels[channelID]
	return pc, ok
}

// getCommandChannels returns a copy of the channel ids linked to a control
// chat. A copy is returned so callers can iterate without holding the lock
// and without racing a concurrent mutation of the backing array.
func (d *Domain) getCommandChannels(chatID int64) []int64 {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	src := d.commandChannels[chatID]
	out := make([]int64, len(src))
	copy(out, src)
	return out
}

func (d *Domain) hasCommandChannels(chatID int64) bool {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	return len(d.commandChannels[chatID]) > 0
}

// snapshotChannelIDs returns a snapshot of all channel IDs in d.channels.
// Use it instead of `for id := range d.channels` when the body may mutate
// the map (e.g. MigrateChannel) or run concurrently with other accessors.
func (d *Domain) snapshotChannelIDs() []int64 {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	out := make([]int64, 0, len(d.channels))
	for id := range d.channels {
		out = append(out, id)
	}
	return out
}

// isChannelLinkedToControlChat reports whether the given protected channel
// is linked to the given control chat (i.e. the control chat is allowed to
// operate on it).
func (d *Domain) isChannelLinkedToControlChat(controlChatID, channelID int64) bool {
	for _, id := range d.getCommandChannels(controlChatID) {
		if id == channelID {
			return true
		}
	}
	return false
}
