package userbot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/telegram/updates"
)

func (d *Domain) Run(
	ctx context.Context,
) error {

	manager := updates.New(
		updates.Config{
			Handler: d.userBot.dispatcher,
		},
	)

	waiter := floodwait.NewWaiter().WithCallback(
		func(ctx context.Context, wait floodwait.FloodWait) {
			d.log.Infof("\nGot FLOOD_WAIT. Will retry after", wait.Duration)
		},
	)

	clientLaunched := make(chan error)
	ctxWaiter, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	go func() {
		err := waiter.Run(
			ctx, func(ctx context.Context) error {
				return d.userBot.client.Run(
					ctx, func(ctx context.Context) error {

						authData, err := d.userBot.client.Auth().Bot(ctx, d.apiKey)
						if err != nil {
							return fmt.Errorf("auth error: %w", err)
						}

						d.log.Infof("auth data: %+v", authData)

						botUser, valid := authData.User.AsNotEmpty()
						if !valid {
							d.userBot.me, err = d.userBot.client.Self(ctx)
							if err != nil {
								return fmt.Errorf("call self error: %w", err)
							}
						} else {
							d.userBot.me = botUser
						}

						clientLaunched <- nil

						return manager.Run(
							ctx, d.userBot.client.API(), d.userBot.me.ID, updates.AuthOptions{
								OnStart: func(ctx context.Context) {
									// fmt.Println("Gaps started...")
								},
							},
						)
					},
				)
			},
		)
		if err != nil {
			clientLaunched <- fmt.Errorf("bot loop finished with error: %w", err)
			d.log.Errorf("bot loop finished with error: %s", err)
			return
		}
		d.log.Infof("bot loop finished")
	}()

	select {
	case err := <-clientLaunched:
		if err != nil {
			d.log.Errorf("user bot instance launch err: %v", err)
			return err
		}
		d.log.Infof("user bot instance launched")
		return nil
	case <-ctxWaiter.Done():
		d.log.Errorf("can't init user bot client =(")
	}

	return errors.New("user bot init error")
}
