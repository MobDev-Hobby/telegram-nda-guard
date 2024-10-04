package kicker

type Option func(d *Domain)

func WithLogger(logger Logger) Option {
	return func(d *Domain) {
		d.log = logger
	}
}

func WithCleanMessages(flag bool) Option {
	return func(d *Domain) {
		d.cleanMessages = flag
	}
}

func WithKeepBanned(flag bool) Option {
	return func(d *Domain) {
		d.keepBanned = flag
	}
}

func WithCleanUnknown(flag bool) Option {
	return func(d *Domain) {
		d.cleanUnknown = flag
	}
}
