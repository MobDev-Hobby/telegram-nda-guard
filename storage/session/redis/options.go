package redis

type SSOpts func(d *Domain)

func WithKeyPrefix(path string) SSOpts {
	return func(d *Domain) {
		d.keyPrefix = path
	}
}

func WithLogger(l Logger) SSOpts {
	return func(d *Domain) {
		d.log = l
	}
}
