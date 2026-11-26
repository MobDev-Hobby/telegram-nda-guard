package scanner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// TestScheduleChangeWakesTheTickerLoop: the ticker loop parks in
// reflect.Select on a snapshot of the registry, so a schedule installed or
// replaced after it parked is invisible until some other case fires. With
// every channel on the same interval that merely delayed a new channel by one
// interval; with per-channel cron entries it is permanent, because replacing a
// schedule removes the cron entry feeding the channel the loop is parked on.
// That channel is then never scanned again - the original incident, reached
// through the new code path.
func TestScheduleChangeWakesTheTickerLoop(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	// No workers: the requests stay in the queue for the test to read.
	d.processingThreads = 0
	d.processRequestChan = make(chan ScanRequest, 4)

	first := make(chan time.Time, 1)
	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{},
	}, WithTickerChan(first)))
	d.channelsMutex.Lock()
	info := d.channels[100]
	info.botOnChannel = true
	d.channels[100] = info
	d.channelsMutex.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.RunUserAccessChecker(ctx)

	// Tick the original schedule so the loop is known to be running and has
	// parked again on a snapshot that contains only this schedule.
	first <- time.Now()
	requireRequest(t, d.processRequestChan, "the original schedule must fire")
	time.Sleep(50 * time.Millisecond)

	// Replace the schedule, as an operator changing the scan time does.
	second := make(chan time.Time, 1)
	d.setSchedule(100, &channelSchedule{kind: scheduleCustom, ch: second})

	second <- time.Now()
	requireRequest(t, d.processRequestChan, "the replacement schedule must fire")
}

// TestScheduleAddedAfterStartFires covers the same wake-up for a channel added
// at runtime (/add, Mini App) while the loop is already parked.
func TestScheduleAddedAfterStartFires(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.processingThreads = 0
	d.processRequestChan = make(chan ScanRequest, 4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.RunUserAccessChecker(ctx)
	time.Sleep(50 * time.Millisecond)

	ticks := make(chan time.Time, 1)
	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{},
	}, WithTickerChan(ticks)))
	d.channelsMutex.Lock()
	info := d.channels[100]
	info.botOnChannel = true
	d.channels[100] = info
	d.channelsMutex.Unlock()

	ticks <- time.Now()
	requireRequest(t, d.processRequestChan, "a channel added after start must fire")
}

func requireRequest(t *testing.T, queue chan ScanRequest, msg string) {
	t.Helper()
	select {
	case req := <-queue:
		require.Equal(t, int64(100), req.channelInfo.id, msg)
	case <-time.After(2 * time.Second):
		t.Fatal(msg)
	}
}

// TestUnusableScheduleFallsBackToInterval: a stored schedule the host cannot
// compile (an unknown zone after a move to an image without tzdata, or a
// hand-edited record) must not leave the channel protected, persisted and
// unscheduled. Scanning on the wrong cadence is recoverable; never scanning is
// the incident this whole change exists to prevent.
func TestUnusableScheduleFallsBackToInterval(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	defer d.StopCron()

	err := d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true,
		ScanReportProcessor: nopReportProcessor{},
		ScanSchedule:        &processors.ScanSchedule{At: "13:05", TZ: "Mars/Olympus"},
	})
	require.NoError(t, err, "an unusable schedule must not fail the whole add")
	require.True(t, d.hasSchedule(100), "the channel must still be scheduled")
	require.Equal(t, scheduleInterval, scheduleKindOf(d, 100))

	_, ok := d.getProtectedChannel(100)
	require.True(t, ok, "the channel stays protected")
}

// An explicit schedule change is different: it is the operator's request, so a
// schedule that cannot be honoured is refused and nothing changes.
func TestExplicitBadScheduleChangeKeepsTheOldSchedule(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	defer d.StopCron()

	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{},
		ScanSchedule: &processors.ScanSchedule{At: "13:05", TZ: "Europe/Moscow"},
	}))
	require.Equal(t, scheduleCron, scheduleKindOf(d, 100))

	err := d.SetChannelSchedule(context.Background(), 100, &processors.ScanSchedule{At: "99:99"}, testAdminID)
	require.Error(t, err)
	require.Equal(t, scheduleCron, scheduleKindOf(d, 100), "the old schedule stays in force")
	stored, _ := storage.get(100)
	require.NotNil(t, stored.ScanSchedule)
	require.Equal(t, "13:05", stored.ScanSchedule.At)
}

// TestScheduleChangeKeepsTheChannelsZone: a client that edits only the time
// and days must not move the channel to another zone. The bundled Mini App
// sends exactly that.
func TestScheduleChangeKeepsTheChannelsZone(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	defer d.StopCron()
	// The service default names a zone; the Domain's fallback does not.
	d.defaultScanSchedule = &processors.ScanSchedule{At: "09:00", TZ: "Europe/Moscow"}

	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{},
	}))

	require.NoError(t, d.SetChannelSchedule(context.Background(), 100,
		&processors.ScanSchedule{At: "10:00"}, testAdminID))

	stored, ok := storage.get(100)
	require.True(t, ok)
	require.NotNil(t, stored.ScanSchedule)
	require.Equal(t, "Europe/Moscow", stored.ScanSchedule.TZ, "the zone must be inherited, not reset to UTC")
}

// TestPartialSettingsKeepsInheritingCleanDefaults: a settings request that
// mentions no cleaning option must leave the channel tracking the service
// defaults rather than freezing a copy of them.
func TestPartialSettingsKeepsInheritingCleanDefaults(t *testing.T) {
	d, _, _, _, storage := newMiniAppDomain(t, true)

	var settings ChannelSettings
	require.NoError(t, json.Unmarshal([]byte(`{"joinRequests":"off"}`), &settings))
	require.NoError(t, d.SetChannelSettings(context.Background(), testChannelID, settings, testAdminID))

	pc, ok := d.getProtectedChannel(testChannelID)
	require.True(t, ok)
	require.Nil(t, pc.CleanOptions, "the channel must keep inheriting the defaults")
	require.Nil(t, storage.records[testChannelID].CleanOptions)

	view, err := d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	require.False(t, view.CustomCleanOptions)
}
