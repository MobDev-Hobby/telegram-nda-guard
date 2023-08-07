package user_bot

import (
	"go.uber.org/zap"
)

type Domain struct {
	appId  int
	appKey string
	log    Logger
}

func New(
	appId int,
	appKey string,
	log Logger,
) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:    logger,
		appId:  appId,
		appKey: appKey,
	}
}
