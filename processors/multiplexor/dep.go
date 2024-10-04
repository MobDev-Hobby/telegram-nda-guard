package multiplexor

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type UserReportProcessor interface {
	ProcessReport(ctx context.Context, report processors.AccessReport)
}
