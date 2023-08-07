package report_processor_multiplexor

import (
	"context"
	"sync"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report report_processor.AccessReport,
) {
	var wg sync.WaitGroup
	for _, processor := range d.processors {
		wg.Add(1)
		go func(processor UserReportProcessor) {
			defer wg.Done()
			processor.ProcessReport(ctx, report)
		}(processor)
	}
	wg.Wait()
}
