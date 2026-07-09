package scanner

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/robfig/cron/v3"

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
		CommandChannelIDs:    pc.CommandChannelIDs,
		AutoScan:             pc.AutoScan,
		AutoClean:            pc.AutoClean,
		AllowClean:           pc.AllowClean,
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

	if d.storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := d.storage.Store(ctx, &channels.ProtectedChannel{
			ID:                   channel.ID,
			CommandChannelIDs:    channel.CommandChannelIDs,
			AutoScan:             channel.AutoScan,
			AutoClean:            channel.AutoClean,
			AllowClean:           channel.AllowClean,
		})
		if err != nil {
			return err
		}
	}
	d.channelsMutex.Lock()
	// Channel cache
	d.channels[channel.ID] = ChannelInfo{
		id:                channel.ID,
		commandChannelIDs: channel.CommandChannelIDs,
	}

	// Protected channels list
	if _, found := d.protectedChannels[channel.ID]; !found {
		d.protectedChannels[channel.ID] = *channel

		// Ticker
		if channel.AutoScan || channel.AutoClean {
			var tickerErr error
			if len(opts) == 0 {
				ticker := time.NewTicker(d.channelAutoScanInterval)
				// registerTicker takes tickerCasesMutex; we already hold
				// channelsMutex but tickerCasesMutex is a different lock and
				// is never taken while holding channelsMutex elsewhere, so
				// there is no lock-ordering deadlock.
				tickerErr = d.registerTicker(ticker.C, channel.ID)
			} else {
				// applyOptions may itself take channelsMutex-free paths and
				// register tickers; unlock to keep the critical section tight.
				d.channelsMutex.Unlock()
				tickerErr = d.applyOptions(opts, channel.ID)
				d.channelsMutex.Lock()
			}
			if tickerErr != nil {
				d.channelsMutex.Unlock()
				return tickerErr
			}
		}
	}

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

func (d *Domain) applyOptions(opts []TickerOption, channelID int64) error {
	for _, opt := range opts {
		options := &TickerOptions{}
		err := opt(options)
		if err != nil {
			d.log.Errorf("Error setting ticker options: %v", err)
		}
		if options.tickerChan == nil {
			continue
		}
		err = d.registerTicker(options.tickerChan, channelID)
		if err != nil {
			return err
		}
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

	// migrate tickers
	d.tickerCasesMutex.Lock()
	defer d.tickerCasesMutex.Unlock()

	for i, tickerChan := range d.tickerCasesChannels {
		if tickerChan == oldChannel {
			d.tickerCasesChannels[i] = newChannel.id
		}
	}

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
		d.removeTickers(channelID)
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
			if err := d.storage.Store(ctx, &channels.ProtectedChannel{
				ID:                protectedChannel.ID,
				CommandChannelIDs: protectedChannel.CommandChannelIDs,
				AutoScan:          protectedChannel.AutoScan,
				AutoClean:         protectedChannel.AutoClean,
				AllowClean:        protectedChannel.AllowClean,
			}); err != nil {
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

func (d *Domain) registerTicker(tickerChan <-chan time.Time, channelID int64) error {
	d.tickerCasesMutex.Lock()
	defer d.tickerCasesMutex.Unlock()

	if len(d.tickerCases) != len(d.tickerCasesChannels) {
		return errors.New("ticker channel count mismatch")
	}

	d.tickerCases = append(
		d.tickerCases,
		reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(tickerChan),
		},
	)
	d.tickerCasesChannels = append(d.tickerCasesChannels, channelID)
	return nil
}

func (d *Domain) removeTickers(channelID int64) {
	d.tickerCasesMutex.Lock()
	defer d.tickerCasesMutex.Unlock()

	for i, tickerChan := range d.tickerCasesChannels {
		if tickerChan == channelID {
			d.tickerCasesChannels = append(d.tickerCasesChannels[:i], d.tickerCasesChannels[i+1:]...)
			d.tickerCases = append(d.tickerCases[:i], d.tickerCases[i+1:]...)
			break
		}
	}
}

// channelHasTicker reports whether a periodic scan/clean ticker is currently
// registered for channelID.
func (d *Domain) channelHasTicker(channelID int64) bool {
	d.tickerCasesMutex.Lock()
	defer d.tickerCasesMutex.Unlock()

	for _, tickerChan := range d.tickerCasesChannels {
		if tickerChan == channelID {
			return true
		}
	}
	return false
}
