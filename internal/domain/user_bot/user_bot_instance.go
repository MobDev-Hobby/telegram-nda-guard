package user_bot

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
)

type UserBotInstance struct {
	userBotDomain     *Domain
	sessionStorage    SessionStorage
	name              string
	dispatcher        tg.UpdateDispatcher
	client            *telegram.Client
	channelUsersCache map[int64][]tg.User
}

type UserBot interface {
	GetChannelUsers(
		ctx context.Context,
		channelId int64,
	) ([]tg.User, error)
}

func (d *Domain) NewBot(
	ctx context.Context,
	sessionStorage SessionStorage,
	authenticator Authenticator,
) UserBot {

	dispatcher := tg.NewUpdateDispatcher()
	instance := &UserBotInstance{
		userBotDomain: d,
		dispatcher:    dispatcher,
		client: telegram.NewClient(
			d.appId,
			d.appKey,
			telegram.Options{
				UpdateHandler:  dispatcher,
				SessionStorage: sessionStorage,
			},
		),
	}

	gaps := updates.New(
		updates.Config{
			Handler: instance.dispatcher,
		},
	)

	waiter := floodwait.NewWaiter().WithCallback(
		func(ctx context.Context, wait floodwait.FloodWait) {
			fmt.Println("\nGot FLOOD_WAIT. Will retry after", wait.Duration)
		},
	)

	clientLaunched := make(chan bool)
	ctxWaiter, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	go func() {
		err := waiter.Run(
			ctx, func(ctx context.Context) error {
				return instance.client.Run(
					ctx, func(ctx context.Context) error {
						flow := auth.NewFlow(
							authenticator,
							auth.SendCodeOptions{},
						)
						if err := instance.client.Auth().IfNecessary(ctx, flow); err != nil {
							return fmt.Errorf("auth error: %s", err)
						}
						clientLaunched <- true
						authenticator.Done()

						self, err := instance.client.Self(ctx)
						if err != nil {
							return fmt.Errorf("call self error: %s", err)
						}

						return gaps.Run(
							ctx, instance.client.API(), self.ID, updates.AuthOptions{
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
			clientLaunched <- false
			d.log.Errorf("bot loop finished with error: %s", err)
			return
		}
		d.log.Infof("bot loop finished")
	}()

	select {
	case ok := <-clientLaunched:
		if ok {
			d.log.Infof("user bot instance launched")
			return instance
		}
		d.log.Errorf("user bot instance launch error")
	case <-ctxWaiter.Done():
		d.log.Errorf("can't init user bot client =(")
	}

	return nil
}
