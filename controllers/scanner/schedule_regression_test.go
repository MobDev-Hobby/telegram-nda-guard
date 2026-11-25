package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// nopReportProcessor satisfies UserReportProcessor; these tests only care
// about which ticker drives a channel, never about the reports themselves.
type nopReportProcessor struct{}

func (nopReportProcessor) ProcessReport(context.Context, processors.AccessReport) {}

// registeredTickerChans returns the ticker channels currently driving
// channelID, so a test can tell a custom/cron schedule from a plain interval
// ticker the code created itself.
func registeredTickerChans(d *Domain, channelID int64) []interface{} {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	out := []interface{}{}
	if schedule, ok := d.schedules[channelID]; ok {
		out = append(out, schedule.ch)
	}
	return out
}

// TestCustomScheduleSurvivesAutoScanToggle reproduces the SPB incident: a
// channel scheduled by cron (production wires every env channel with
// WithCronScheduledChannel) silently loses that schedule as soon as AutoScan
// is switched off and on again, because the schedule registry could not
// tell a cron-backed ticker from an interval one.
func TestCustomScheduleSurvivesAutoScanToggle(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	cronTicks := make(chan time.Time, 1)

	channel := &ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{}}
	if err := d.AddProtectedChannel(channel, WithTickerChan(cronTicks)); err != nil {
		t.Fatal(err)
	}
	want := []interface{}{(<-chan time.Time)(cronTicks)}
	if got := registeredTickerChans(d, 100); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("cron schedule not registered: %v", got)
	}

	ctx := context.Background()
	// An operator switches AutoScan off (Mini App settings, /setflag, or a
	// settings PUT that omitted autoScan) and then back on.
	if err := d.applyFlagToggle(ctx, 100, flagAutoScan); err != nil {
		t.Fatal(err)
	}
	if err := d.applyFlagToggle(ctx, 100, flagAutoScan); err != nil {
		t.Fatal(err)
	}
	if !d.protectedChannels[100].AutoScan {
		t.Fatal("AutoScan should be back on")
	}

	got := registeredTickerChans(d, 100)
	if len(got) != 1 {
		t.Fatalf("channel 100 has %d tickers, want 1: %v", len(got), got)
	}
	if got[0] != want[0] {
		t.Errorf("cron schedule was replaced by an interval ticker: the channel no longer scans on its cron")
	}
}

// TestSettingsUpdateKeepsCustomSchedule covers the path that actually fires in
// production: every access-check cycle calls updateProtectedChannel to refresh
// the channel title, and the Mini App settings PUT uses the same helper.
// Neither intends to touch the schedule.
func TestSettingsUpdateKeepsCustomSchedule(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	cronTicks := make(chan time.Time, 1)

	channel := &ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{}}
	if err := d.AddProtectedChannel(channel, WithTickerChan(cronTicks)); err != nil {
		t.Fatal(err)
	}
	want := (<-chan time.Time)(cronTicks)

	ctx := context.Background()
	// A settings payload that leaves autoScan at its zero value turns the
	// automation off; the operator then turns it back on.
	if err := d.updateProtectedChannel(ctx, 100, func(pc *ProtectedChannel) { pc.AutoScan = false }); err != nil {
		t.Fatal(err)
	}
	if err := d.updateProtectedChannel(ctx, 100, func(pc *ProtectedChannel) { pc.AutoScan = true }); err != nil {
		t.Fatal(err)
	}

	got := registeredTickerChans(d, 100)
	if len(got) != 1 || got[0] != want {
		t.Errorf("custom schedule lost after a settings round-trip: %v", got)
	}
}

// TestScheduleOutlivesDisabledAutomation pins the invariant the fix rests on:
// a protected channel is scheduled whatever its automation flags say, so the
// ticker loop decides per tick whether there is work. Without it, switching
// automation off destroys the schedule.
func TestScheduleOutlivesDisabledAutomation(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	cronTicks := make(chan time.Time, 1)

	// Added with automation off, as /add and the Mini App do.
	channel := &ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}}
	if err := d.AddProtectedChannel(channel, WithTickerChan(cronTicks)); err != nil {
		t.Fatal(err)
	}
	if !d.hasSchedule(100) {
		t.Fatal("a protected channel must be scheduled even with automation off")
	}
	if got := registeredTickerChans(d, 100); len(got) != 1 || got[0] != (<-chan time.Time)(cronTicks) {
		t.Fatalf("custom schedule not kept: %v", got)
	}
}

// TestDetachingChannelDropsSchedule covers the one place a schedule should
// disappear: the channel is no longer protected from any chat.
func TestDetachingChannelDropsSchedule(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	if err := d.AddProtectedChannel(&ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}}); err != nil {
		t.Fatal(err)
	}
	if !d.hasSchedule(100) {
		t.Fatal("channel should be scheduled")
	}
	if err := d.CleanProtectedChannel(100, 5); err != nil {
		t.Fatal(err)
	}
	if d.hasSchedule(100) {
		t.Error("a detached channel must not keep a schedule")
	}
}

// TestMigrationMovesSchedule checks a chat upgraded to a supergroup keeps its
// own schedule under the new id instead of being left unscheduled.
func TestMigrationMovesSchedule(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	cronTicks := make(chan time.Time, 1)
	if err := d.AddProtectedChannel(
		&ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{}},
		WithTickerChan(cronTicks),
	); err != nil {
		t.Fatal(err)
	}
	if err := d.MigrateChannel(100, 200); err != nil {
		t.Fatal(err)
	}
	if d.hasSchedule(100) {
		t.Error("the old id must not stay scheduled")
	}
	if got := registeredTickerChans(d, 200); len(got) != 1 || got[0] != (<-chan time.Time)(cronTicks) {
		t.Errorf("schedule did not follow the migration: %v", got)
	}
}

// TestBadScheduleIsRejected: a malformed schedule used to be logged and
// swallowed, leaving the channel protected but never scanned.
func TestBadScheduleIsRejected(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	err := d.AddProtectedChannel(
		&ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}},
		WithCron("not a cron spec"),
	)
	if err == nil {
		t.Fatal("expected an error for an invalid cron spec")
	}
}
