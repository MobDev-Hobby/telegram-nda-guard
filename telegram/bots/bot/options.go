package bot

import "context"

type Options struct {
	*Domain
	initCtx context.Context
}
type TelegramBotOption func(d *Options)

func WithLogger(logger Logger) TelegramBotOption {
	return func(d *Options) {
		d.log = logger
	}
}

func WithInitContext(ctx context.Context) TelegramBotOption {
	return func(d *Options) {
		d.initCtx = ctx
	}
}
