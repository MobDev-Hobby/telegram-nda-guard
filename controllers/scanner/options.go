package scanner

import "time"

type ProcessorOption func(*Domain)

func WithLogger(log Logger) func(*Domain) {
	if log == nil {
		panic("log is nil")
	}
	return func(d *Domain) {
		d.log = log
	}
}

func WithSetAdminKey(key string) func(*Domain) {
	if len(key) < 32 {
		panic("insecure root key")
	}
	return func(d *Domain) {
		d.setAdminHash = &key
	}
}

func WithChannelAutoScanInterval(interval time.Duration) func(*Domain) {
	if interval < 10*time.Second {
		panic("too small auto scan interval can occur ban")
	}
	return func(d *Domain) {
		d.channelAutoScanInterval = interval
	}
}

func WithCheckAccessInterval(interval time.Duration) func(*Domain) {
	if interval < 10*time.Second {
		panic("too small auto scan interval can occur ban")
	}
	return func(d *Domain) {
		d.accessCheckInterval = interval
	}
}

func WithOwnerChatID(ownerChatID int64) func(*Domain) {
	if ownerChatID <= 0 {
		panic("invalid ownerChatID")
	}
	return func(d *Domain) {
		d.adminUserChatID = ownerChatID
	}
}

func WithChannels(channels []ProtectedChannel) func(*Domain) {
	return func(d *Domain) {
		for _, channel := range channels {
			channel := channel
			err := d.AddProtectedChannel(&channel)
			if err != nil {
				panic("cannot add channel to domain")
			}
		}
	}
}

func WithCustomScheduledChannel(channel ProtectedChannel, tickerChan chan time.Time) func(*Domain) {
	return func(d *Domain) {
		err := d.AddProtectedChannel(&channel, WithTickerChan(tickerChan))
		if err != nil {
			panic("cannot add channel to domain")
		}
	}
}

func WithCronScheduledChannel(channel ProtectedChannel, cronString string) func(*Domain) {
	return func(d *Domain) {
		err := d.AddProtectedChannel(&channel, WithCron(cronString))
		if err != nil {
			panic("cannot add channel to domain")
		}
	}
}

func WithTaskDelayInterval(interval time.Duration) func(*Domain) {
	return func(d *Domain) {
		d.taskDelayInterval = interval
	}
}

func WithNProcessingThreads(threads int) func(*Domain) {
	if threads <= 0 {
		panic("invalid threads quantity, bot will not work")
	}
	return func(d *Domain) {
		d.processingThreads = threads
	}
}

func WithDefaultScanProcessor(processor UserReportProcessor) func(*Domain) {
	return func(d *Domain) {
		d.defaultScanProcessor = processor
	}
}

func WithDefaultCleanProcessor(processor UserReportProcessor) func(*Domain) {
	return func(d *Domain) {
		d.defaultCleanProcessor = processor
	}
}

func WithDefaultAccessChecker(checker CheckUserAccess) func(*Domain) {
	return func(d *Domain) {
		d.defaultAccessChecker = checker
	}
}
