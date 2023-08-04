package user_bot

import (
	"go.uber.org/zap"
)

type Domain struct {
	appId  int
	appKey string
	log    *zap.SugaredLogger
}

func New(
	appId int,
	appKey string,
	log *zap.SugaredLogger,
) *Domain {
	logger := zap.NewNop().Sugar()
	if log != nil {
		logger = log
	}
	return &Domain{
		log:    logger,
		appId:  appId,
		appKey: appKey,
	}
}
