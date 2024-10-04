package file

type SSOpts func(d *Domain)

func WithStoragePath(path string) SSOpts {
	return func(d *Domain) {
		d.dir = path
	}
}

func WithLogger(l Logger) SSOpts {
	return func(d *Domain) {
		d.log = l
	}
}
