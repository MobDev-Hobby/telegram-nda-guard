package multiplexor

import "go.uber.org/zap"

type Domain struct {
	log        Logger
	processors []UserReportProcessor
}

func New(log Logger, processors ...UserReportProcessor) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:        logger,
		processors: processors,
	}
}
