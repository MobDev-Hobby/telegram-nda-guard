package reporter

type Option func(d *Domain)

func WithLogger(logger Logger) Option {
	return func(d *Domain) {
		d.log = logger
	}
}
