package multiplexor

import (
	"context"
	"sync"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

func (d *Domain) ProcessReport(
	ctx context.Context,
	report processors.AccessReport,
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
