package scanner

import (
	"context"
	"errors"
	"fmt"
	"html"
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

// WithCron schedules a channel from a raw cron expression.
//
// Deprecated: it starts a scheduler per channel that nothing can stop, and the
// expression is read in the host's local zone unless it carries its own
// CRON_TZ= prefix - in a container that is whatever the image sets. Prefer
// WithDefaultScanSchedule, or a per-channel ScanSchedule, which name their
// zone and share one scheduler.
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
		ScanSchedule:         pc.ScanSchedule,
		Managers:             pc.Managers,
		LastCheck:            pc.LastCheck,
		JoinRequestMode:      pc.JoinRequestMode,
		AccessChecker:        d.defaultAccessChecker,
		ScanReportProcessor:  d.defaultScanProcessor,
		CleanReportProcessor: d.defaultCleanProcessor,
	})
}

// mergeStrategy reconciles a channel that is already protected with one being
// added again. The two strategies differ in who wins: see
// mergeProtectedChannel and mergeRestoredChannel.
type mergeStrategy func(existing, added ProtectedChannel) ProtectedChannel

func (d *Domain) AddProtectedChannel(channel *ProtectedChannel, opts ...TickerOption) error {
	return d.addProtectedChannel(channel, mergeProtectedChannel, opts...)
}

func (d *Domain) addProtectedChannel(channel *ProtectedChannel, merge mergeStrategy, opts ...TickerOption) error {
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
		record = merge(existing, *channel)
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
	d.protectedChannels[record.ID] = record
	channel = &record

	// Schedule. A protected channel always gets one, whatever its automation
	// flags say right now: the ticker loop checks AutoScan/AutoClean per tick,
	// and a schedule that comes and goes with the flags cannot survive being
	// switched off (a caller-supplied cron would be lost for good).
	// The schedule helpers take their own locks and read protectedChannels,
	// so release channelsMutex around them.
	d.channelsMutex.Unlock()
	if len(opts) > 0 {
		// An explicit ticker option from the embedding application wins over
		// any configured schedule. A malformed one is a wiring mistake, not
		// runtime data, so it fails the add and surfaces at startup.
		if err := d.applyOptions(opts, record.ID); err != nil {
			// channelsMutex is already released here; nothing to unwind.
			return err
		}
	} else if err := d.installSchedule(record.ID); err != nil {
		// Here the channel is already protected and persisted, and the
		// schedule came from stored or configured data: a zone missing from
		// the image, a hand-edited record. Leaving it protected and never
		// scanned is the failure this whole area exists to prevent, so fall
		// back to the interval and say so loudly. The wrong cadence is
		// recoverable; silence is not.
		d.log.Errorf("channel %d: %s; falling back to scanning every %s",
			record.ID, err, d.channelAutoScanInterval)
		d.ensureIntervalSchedule(record.ID)
		if !d.hasSchedule(record.ID) {
			return fmt.Errorf("channel %d cannot be scheduled: %w", record.ID, err)
		}
		d.NotifyAdmin(context.Background(), fmt.Sprintf(
			"Channel <b>%d</b> has an unusable scan schedule (%s).\nIt is being scanned every %s instead. Fix its schedule in the app.",
			record.ID, html.EscapeString(err.Error()), d.channelAutoScanInterval,
		))
	}
	d.channelsMutex.Lock()

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

// mergeRestoredChannel reconciles a record loaded from storage with one the
// embedding application wired from its configuration (e.g. the CHANNELS env
// var) before Run().
//
// It is the mirror image of mergeProtectedChannel: there, a channel being
// added again must not clobber the settings it already has, so existing wins.
// Here, the stored record IS the operator's settings - what they last chose in
// the Mini App or with /setflag - so it wins over the configured defaults.
// Keeping the configuration authoritative meant a channel switched off by an
// operator came back on at the next restart, and one whose notification or
// cleaning preferences differed from the defaults silently lost them.
//
// Wiring that is never persisted (checkers, report processors) comes from the
// configured record; control chats and managers are the union of both.
func mergeRestoredChannel(configured, stored ProtectedChannel) ProtectedChannel {
	merged := configured

	merged.AutoScan = stored.AutoScan
	merged.AutoClean = stored.AutoClean
	merged.AllowClean = stored.AllowClean
	merged.CleanOptions = stored.CleanOptions
	merged.ScanSchedule = stored.ScanSchedule
	merged.JoinRequestMode = stored.JoinRequestMode
	merged.Notifications = stored.Notifications
	merged.LastCheck = stored.LastCheck
	merged.LastNotifiedAt = stored.LastNotifiedAt

	if stored.Title != "" {
		merged.Title = stored.Title
	}
	if stored.ChatType != "" {
		merged.ChatType = stored.ChatType
	}
	if stored.AddedBy != 0 {
		merged.AddedBy, merged.AddedByName, merged.AddedAt = stored.AddedBy, stored.AddedByName, stored.AddedAt
	}

	merged.CommandChannelIDs = unionIDs(configured.CommandChannelIDs, stored.CommandChannelIDs)
	merged.Managers = unionIDs(configured.Managers, stored.Managers)
	return merged
}

func unionIDs(base, added []int64) []int64 {
	out := slices.Clone(base)
	for _, id := range added {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// restoreProtectedChannel brings a persisted channel back after a restart,
// letting the stored settings win over anything the application configured.
func (d *Domain) restoreProtectedChannel(
	stored *channels.ProtectedChannel,
	scanProcessor, cleanProcessor UserReportProcessor,
	accessChecker CheckUserAccess,
) error {
	if stored == nil {
		return nil
	}
	record := ProtectedChannel{
		ID:                stored.ID,
		Title:             stored.Title,
		ChatType:          stored.ChatType,
		AddedBy:           stored.AddedBy,
		AddedByName:       stored.AddedByName,
		AddedAt:           stored.AddedAt,
		Notifications:     stored.Notifications,
		LastNotifiedAt:    stored.LastNotifiedAt,
		CommandChannelIDs: stored.CommandChannelIDs,
		AutoScan:          stored.AutoScan,
		AutoClean:         stored.AutoClean,
		AllowClean:        stored.AllowClean,
		CleanOptions:      stored.CleanOptions,
		ScanSchedule:      stored.ScanSchedule,
		Managers:          stored.Managers,
		LastCheck:         stored.LastCheck,
		JoinRequestMode:   stored.JoinRequestMode,

		AccessChecker:        accessChecker,
		ScanReportProcessor:  scanProcessor,
		CleanReportProcessor: cleanProcessor,
	}
	return d.addProtectedChannel(&record, mergeRestoredChannel)
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

// CleanProtectedChannel detaches channelID from one control chat, and when
// that was its last one stops protecting it altogether.
//
// It holds channelsMutex throughout: it reads and writes protectedChannels and
// commandChannels, which the rights checker, the scan workers, the ticker loop
// and the update handlers all touch. It used to take no lock at all, and that
// matters more now that AddProtectedChannel releases channelsMutex to install
// a schedule - a detach landing in that window could delete the record while
// the add was mid-flight and have the add put the caches back for a channel
// that is no longer protected.
func (d *Domain) CleanProtectedChannel(channelID int64, commandChannelId int64) error {
	d.channelsMutex.Lock()
	defer d.channelsMutex.Unlock()

	protectedChannel, found := d.protectedChannels[channelID]
	if !found {
		return nil
	}

	// Copy before editing: the slice header is shared with the stored record
	// and with the caller's view of it.
	commandChannels := slices.Clone(protectedChannel.CommandChannelIDs)
	for i, commandChannel := range commandChannels {
		if commandChannel == commandChannelId {
			commandChannels = append(commandChannels[:i], commandChannels[i+1:]...)
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

	controlledChannels := slices.Clone(d.commandChannels[commandChannelId])
	for i, chanID := range controlledChannels {
		if chanID == channelID {
			controlledChannels = append(controlledChannels[:i], controlledChannels[i+1:]...)
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
	var scanSchedule *processors.ScanSchedule
	if pc.ScanSchedule != nil {
		schedule := *pc.ScanSchedule
		schedule.Weekdays = slices.Clone(pc.ScanSchedule.Weekdays)
		scanSchedule = &schedule
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
		ScanSchedule:      scanSchedule,
		Managers:          append([]int64(nil), pc.Managers...),
		LastCheck:         pc.LastCheck,
		JoinRequestMode:   pc.JoinRequestMode,
	}
}
