package userbot

import (
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

type Domain struct {
	log            Logger
	sessionStorage SessionStorage
	apiKey         string
	userBot        *UserBotInstance
}

type UserBotInstance struct {
	dispatcher tg.UpdateDispatcher
	client     *telegram.Client
	me         *tg.User
}

func New(
	appID int,
	appKey string,
	apiKey string,
	sessionStorage SessionStorage,
	opts ...UserBotOption,
) *Domain {

	dispatcher := tg.NewUpdateDispatcher()

	d := &Options{
		name: "example_bot",
		Domain: &Domain{
			apiKey:         apiKey,
			log:            Logger(zap.NewNop().Sugar()),
			sessionStorage: sessionStorage,
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	d.userBot = &UserBotInstance{
		dispatcher: dispatcher,
		client: telegram.NewClient(
			appID,
			appKey,
			telegram.Options{
				UpdateHandler:  dispatcher,
				SessionStorage: d.getSessionStorage(d.name),
			},
		),
	}

	return d.Domain
}
