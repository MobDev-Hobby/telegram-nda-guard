package userbot

type UserBotOption func(d *Options)

type Options struct {
	*Domain
	name string
}

func WithLogger(l Logger) UserBotOption {
	return func(d *Options) {
		d.log = l
	}
}

func WithStorageKey(name string) UserBotOption {
	return func(d *Options) {
		d.name = name
	}
}
