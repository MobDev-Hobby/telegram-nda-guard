package scanner

import (
	"errors"
	"reflect"
	"time"

	"github.com/robfig/cron/v3"
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
				tickerChan <- time.Now()
			},
		)
		if err != nil {
			return err
		}
		opts.tickerChan = tickerChan
		return nil
	}
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

	// Channel cache
	d.channels[channel.ID] = ChannelInfo{
		id:                channel.ID,
		commandChannelIDs: channel.CommandChannelIDs,
	}

	// Protected channels list
	d.protectedChannels[channel.ID] = *channel

	// Ticker
	if channel.AutoScan || channel.AutoClean {
		var err error
		if len(opts) == 0 {
			ticker := time.NewTicker(d.channelAutoScanInterval)
			err = d.registerTicker(ticker.C, channel.ID)
		} else {
			err = d.applyOptions(opts, channel.ID)
		}
		if err != nil {
			return err
		}
	}

	// Command channels cache
	for _, commandChannelID := range channel.CommandChannelIDs {
		d.commandChannels[commandChannelID] = append(
			d.commandChannels[commandChannelID],
			channel.ID,
		)
	}

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
