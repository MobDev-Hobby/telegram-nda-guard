package scanner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

type TickerOption func(opts *TickerOptions) error
type TickerOptions struct {
	tickerChan <-chan time.Time
}

func WithTicker(ticker time.Ticker) TickerOption {
	return func(opts *TickerOptions) error {
		opts.tickerChan = ticker.C
		return nil
	}
}

func WithTickerChan(tickerChan chan time.Time) TickerOption {
	return func(opts *TickerOptions) error {
		opts.tickerChan = tickerChan
		return nil
	}
}

func WithCron(cronString string) TickerOption {
	return func(opts *TickerOptions) error {
		c := cron.New()
		tickerChan := make(chan time.Time, 1)
		_, err := c.AddFunc(
			cronString, func() {
				select {
				case tickerChan <- time.Now():
				default:
					// drop if the buffer is full; a pending tick is enough
				}
			},
		)
		if err != nil {
			return err
		}
		// c.Start() was missing, so the scheduler never fired and
		// WithCronScheduledChannel produced a channel that never ticked.
		c.Start()
		opts.tickerChan = tickerChan
		return nil
	}
}

func (d *Domain) AddDefaultProtectedChannel(pc *ProtectedChannel) error {
	if d.defaultAccessChecker == nil || d.defaultCleanProcessor == nil || d.defaultScanProcessor == nil {
		return errors.New("you need to set defaultAccessChecker, defaultCleanProcessor and defaultScanProcessor")
	}
	return d.AddProtectedChannel(&ProtectedChannel{
		ID:                   pc.ID,
		Title:                pc.Title,
		ChatType:             pc.ChatType,
		AddedBy:              pc.AddedBy,
		AddedByName:          pc.AddedByName,
		AddedAt:              pc.AddedAt,
		Notifications:        pc.Notifications,
		LastNotifiedAt:       pc.LastNotifiedAt,
		CommandChannelIDs:    pc.CommandChannelIDs,
		AutoScan:             pc.AutoScan,
		AutoClean:            pc.AutoClean,
		AllowClean:           pc.AllowClean,
		CleanOptions:         pc.CleanOptions,
		Managers:             pc.Managers,
		LastCheck:            pc.LastCheck,
		JoinRequestMode:      pc.JoinRequestMode,
		AccessChecker:        d.defaultAccessChecker,
		ScanReportProcessor:  d.defaultScanProcessor,
		CleanReportProcessor: d.defaultCleanProcessor,
	})
}

func (d *Domain) AddProtectedChannel(channel *ProtectedChannel, opts ...TickerOption) error {
	if channel == nil {
		return nil
	}

	if channel.AutoScan && channel.ScanReportProcessor == nil {
		return errors.New("you need to set ScanReportProcessor before AutoScan")
	}

	if channel.AutoClean && channel.CleanReportProcessor == nil {
		return errors.New("you need to set CleanReportProcessor before AutoClean")
	}

	if channel.ID == 0 {
		return errors.New("invalid id")
	}

	// Adding a channel that is already protected (e.g. /add from another chat)
	// only adds control chats and managers; everything else stays as is.
	d.channelsMutex.Lock()
	existing, found := d.protectedChannels[channel.ID]
	record := *channel
	if found {
		record = mergeProtectedChannel(existing, *channel)
	}

	if d.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := d.storage.Store(ctx, storageRecord(record))
		if err != nil {
			d.channelsMutex.Unlock()
			return err
		}
	}

	// Protected channels list
	if existing, found := d.protectedChannels[channel.ID]; found {
		merged := mergeProtectedChannel(existing, *channel)
		d.protectedChannels[channel.ID] = merged
		channel = &merged
	} else {
		d.protectedChannels[channel.ID] = *channel
	}

	// Schedule. A protected channel always gets one, whatever its automation
	// flags say right now: the ticker loop checks AutoScan/AutoClean per tick,
	// and a schedule that comes and goes with the flags cannot survive being
	// switched off (a caller-supplied cron would be lost for good).
	if len(opts) > 0 {
		// applyOptions takes scheduleMutex, a different lock; unlock
		// channelsMutex anyway to keep the critical section tight.
		d.channelsMutex.Unlock()
		err := d.applyOptions(opts, channel.ID)
		d.channelsMutex.Lock()
		if err != nil {
			d.channelsMutex.Unlock()
			return err
		}
	} else {
		d.ensureIntervalSchedule(channel.ID)
	}

	// Channel cache
	info := d.channels[channel.ID]
	info.id = channel.ID
	if channel.Title != "" {
		info.title = channel.Title
	}
	if channel.ChatType != "" {
		info.chatType = channel.ChatType
	}
	info.commandChannelIDs = channel.CommandChannelIDs
	d.channels[channel.ID] = info

	// Command channels cache
	for _, commandChannelID := range channel.CommandChannelIDs {
		haveChannel := false
		for i := range d.commandChannels[commandChannelID] {
			if d.commandChannels[commandChannelID][i] == channel.ID {
				haveChannel = true
				break
			}
		}
		if haveChannel {
			continue
		}
		d.commandChannels[commandChannelID] = append(
			d.commandChannels[commandChannelID],
			channel.ID,
		)
	}
	d.channelsMutex.Unlock()

	d.log.Infof("Added protected channel %d/%v", channel.ID, channel.CommandChannelIDs)

	return nil
}

// mergeProtectedChannel adds the control chats and managers of added to
// existing and keeps every other field of existing.
func mergeProtectedChannel(existing, added ProtectedChannel) ProtectedChannel {
	merged := existing
	if merged.Title == "" {
		merged.Title = added.Title
	}
	if merged.ChatType == "" {
		merged.ChatType = added.ChatType
	}
	if merged.AddedBy == 0 {
		merged.AddedBy, merged.AddedByName, merged.AddedAt = added.AddedBy, added.AddedByName, added.AddedAt
	} else if merged.AddedBy == added.AddedBy && merged.AddedByName == "" {
		merged.AddedByName = added.AddedByName
	}
	merged.CommandChannelIDs = slices.Clone(existing.CommandChannelIDs)
	for _, id := range added.CommandChannelIDs {
		if !slices.Contains(merged.CommandChannelIDs, id) {
			merged.CommandChannelIDs = append(merged.CommandChannelIDs, id)
		}
	}
	merged.Managers = slices.Clone(existing.Managers)
	for _, id := range added.Managers {
		if !slices.Contains(merged.Managers, id) {
			merged.Managers = append(merged.Managers, id)
		}
	}
	return merged
}

// applyOptions installs the schedule described by opts. A channel has one
// schedule, so when several options supply one the last wins.
func (d *Domain) applyOptions(opts []TickerOption, channelID int64) error {
	installed := false
	for _, opt := range opts {
		options := &TickerOptions{}
		err := opt(options)
		if err != nil {
			// A malformed schedule must not leave the channel silently
			// unscheduled; it used to be logged and swallowed.
			return fmt.Errorf("channel %d schedule: %w", channelID, err)
		}
		if options.tickerChan == nil {
			continue
		}
		d.setSchedule(channelID, &channelSchedule{kind: scheduleCustom, ch: options.tickerChan})
		installed = true
	}
	if !installed {
		d.ensureIntervalSchedule(channelID)
	}
	return nil
}

func (d *Domain) MigrateChannel(migratedFromID, migratedToID int64) error {
	d.channelsMutex.Lock()
	defer d.channelsMutex.Unlock()

	newChannel := d.channels[migratedFromID]
	oldChannel := migratedFromID
	newChannel.migratedFrom = &oldChannel
	newChannel.id = migratedToID

	// Migrate channels cache
	d.channels[newChannel.id] = newChannel
	delete(d.channels, migratedFromID)

	// Migrate protected channels
	newProtectedChannel := d.protectedChannels[migratedFromID]
	newProtectedChannel.ID = migratedToID
	d.protectedChannels[newChannel.id] = newProtectedChannel
	delete(d.protectedChannels, migratedFromID)

	// Migrate control channels
	for _, commandChannelID := range newChannel.commandChannelIDs {
		controlledChannels := d.commandChannels[commandChannelID]
		for i, chanID := range controlledChannels {
			if chanID == oldChannel {
				controlledChannels[i] = newChannel.id
			}
		}
		d.commandChannels[commandChannelID] = controlledChannels
	}

	// migrate the schedule
	d.migrateSchedule(oldChannel, newChannel.id)

	return nil
}

func (d *Domain) CleanProtectedChannel(channelID int64, commandChannelId int64) error {

	protectedChannel, found := d.protectedChannels[channelID]
	if !found {
		return nil
	}

	commandChannels := protectedChannel.CommandChannelIDs
	for i, commandChannel := range commandChannels {
		if commandChannel == commandChannelId {
			commandChannels[i] = commandChannels[len(commandChannels)-1]
			commandChannels = commandChannels[:len(commandChannels)-1]
			break
		}
	}

	if len(commandChannels) == 0 {
		d.dropSchedule(channelID)
		delete(d.protectedChannels, channelID)
		// The channel is fully detached: also remove its persisted record so it
		// does not reappear after a restart. Drop is idempotent.
		if d.storage != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := d.storage.Drop(ctx, channelID); err != nil {
				d.log.Errorf("can't drop channel %d from storage: %s", channelID, err)
				return err
			}
		}
	} else {
		protectedChannel.CommandChannelIDs = commandChannels
		d.protectedChannels[channelID] = protectedChannel
		// The set of controlling chats changed: persist the updated record.
		if d.storage != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := d.storage.Store(ctx, storageRecord(protectedChannel)); err != nil {
				d.log.Errorf("can't update channel %d in storage: %s", channelID, err)
				return err
			}
		}
	}

	controlledChannels := d.commandChannels[commandChannelId]
	for i, chanID := range controlledChannels {
		if chanID == channelID {
			controlledChannels[i] = controlledChannels[len(controlledChannels)-1]
			controlledChannels = controlledChannels[:len(controlledChannels)-1]
			break
		}
	}
	d.commandChannels[commandChannelId] = controlledChannels

	return nil
}

// storageRecord converts a protected channel into its persisted form.
func storageRecord(pc ProtectedChannel) *channels.ProtectedChannel {
	var cleanOptions *processors.CleanOptions
	if pc.CleanOptions != nil {
		opts := *pc.CleanOptions
		cleanOptions = &opts
	}
	return &channels.ProtectedChannel{
		ID:                pc.ID,
		Title:             pc.Title,
		ChatType:          pc.ChatType,
		AddedBy:           pc.AddedBy,
		AddedByName:       pc.AddedByName,
		AddedAt:           pc.AddedAt,
		Notifications:     pc.Notifications,
		LastNotifiedAt:    pc.LastNotifiedAt,
		CommandChannelIDs: pc.CommandChannelIDs,
		AutoScan:          pc.AutoScan,
		AutoClean:         pc.AutoClean,
		AllowClean:        pc.AllowClean,
		CleanOptions:      cleanOptions,
		Managers:          append([]int64(nil), pc.Managers...),
		LastCheck:         pc.LastCheck,
		JoinRequestMode:   pc.JoinRequestMode,
	}
}
