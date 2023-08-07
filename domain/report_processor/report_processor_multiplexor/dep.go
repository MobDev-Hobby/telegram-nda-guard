package report_processor_multiplexor

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
)

type Logger interfaces.Logger

type UserReportProcessor interface {
	ProcessReport(ctx context.Context, report report_processor.AccessReport)
}
