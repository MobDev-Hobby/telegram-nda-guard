package scanner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

func TestScanScheduleCronSpec(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schedule processors.ScanSchedule
		fallback string
		want     string
	}{
		{
			name:     "weekdays",
			schedule: processors.ScanSchedule{At: "13:05", Weekdays: weekdaysMonToFri(), TZ: "Europe/Moscow"},
			want:     "CRON_TZ=Europe/Moscow 5 13 * * 1,2,3,4,5",
		},
		{
			name:     "every day",
			schedule: processors.ScanSchedule{At: "09:00", TZ: "UTC"},
			want:     "CRON_TZ=UTC 0 9 * * *",
		},
		{
			name:     "zone falls back to the service default",
			schedule: processors.ScanSchedule{At: "23:59"},
			fallback: "Asia/Tashkent",
			want:     "CRON_TZ=Asia/Tashkent 59 23 * * *",
		},
		{
			name:     "sunday is 0",
			schedule: processors.ScanSchedule{At: "00:30", Weekdays: []time.Weekday{time.Sunday}, TZ: "UTC"},
			want:     "CRON_TZ=UTC 30 0 * * 0",
		},
		{
			name:     "days are deduplicated and ordered",
			schedule: processors.ScanSchedule{At: "07:00", Weekdays: []time.Weekday{time.Friday, time.Monday, time.Friday}, TZ: "UTC"},
			want:     "CRON_TZ=UTC 0 7 * * 1,5",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cronSpec(tc.schedule, tc.fallback)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestScanScheduleRejectsNonsense(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schedule processors.ScanSchedule
	}{
		{"empty time", processors.ScanSchedule{}},
		{"no colon", processors.ScanSchedule{At: "1305"}},
		{"hour out of range", processors.ScanSchedule{At: "24:00"}},
		{"minute out of range", processors.ScanSchedule{At: "13:60"}},
		{"not a number", processors.ScanSchedule{At: "ab:cd"}},
		{"unknown zone", processors.ScanSchedule{At: "13:05", TZ: "Mars/Olympus"}},
		{"weekday out of range", processors.ScanSchedule{At: "13:05", Weekdays: []time.Weekday{9}}},
		{"cron injection", processors.ScanSchedule{At: "13:05", TZ: "UTC * * * * *"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cronSpec(tc.schedule, "UTC")
			assert.Error(t, err)
		})
	}
}

// The compiled spec must be accepted by the cron library itself, not just by
// our own validation.
func TestScanScheduleSpecIsParseable(t *testing.T) {
	spec, err := cronSpec(processors.ScanSchedule{At: "13:05", Weekdays: weekdaysMonToFri(), TZ: "Europe/Moscow"}, "")
	require.NoError(t, err)
	next, err := nextRun(spec, time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	// 2026-11-25 is a Wednesday; 13:05 Moscow is 10:05 UTC.
	assert.Equal(t, time.Date(2026, 11, 25, 10, 5, 0, 0, time.UTC), next.UTC())
}

func TestReportBoundary(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	// A scan at 13:05 Moscow with the summary released from 13:00 is past the
	// boundary, so the day's summary goes out.
	now := time.Date(2026, 11, 25, 13, 5, 0, 0, msk)
	boundary, err := reportBoundary("13:00", msk, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 25, 13, 0, 0, 0, msk), boundary)

	// Before the release time, the boundary is yesterday's, so a summary sent
	// yesterday afternoon still counts as sent for this period.
	now = time.Date(2026, 11, 25, 9, 0, 0, 0, msk)
	boundary, err = reportBoundary("13:00", msk, now)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 11, 24, 13, 0, 0, 0, msk), boundary)

	// Exactly at the release time counts as due.
	now = time.Date(2026, 11, 25, 13, 0, 0, 0, msk)
	boundary, err = reportBoundary("13:00", msk, now)
	require.NoError(t, err)
	assert.Equal(t, now, boundary)

	_, err = reportBoundary("nope", msk, now)
	assert.Error(t, err)
}

// TestSetChannelScheduleRebuildsTheCronEntry: changing a channel's scan time
// must take effect without a restart, persist, and survive a flag toggle.
func TestSetChannelScheduleRebuildsTheCronEntry(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.scheduleTZ = "Europe/Moscow"
	defer d.StopCron()

	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5}, AutoScan: true, ScanReportProcessor: nopReportProcessor{},
	}))
	// No schedule anywhere yet, so the channel is on the interval ticker.
	assert.Equal(t, scheduleInterval, scheduleKindOf(d, 100))

	ctx := context.Background()
	require.NoError(t, d.SetChannelSchedule(ctx, 100, &processors.ScanSchedule{
		At: "09:30", Weekdays: weekdaysMonToFri(),
	}, testAdminID))
	assert.Equal(t, scheduleCron, scheduleKindOf(d, 100))

	stored, ok := storage.get(100)
	require.True(t, ok)
	require.NotNil(t, stored.ScanSchedule)
	assert.Equal(t, "09:30", stored.ScanSchedule.At)

	// Switching automation off and on must not cost the channel its schedule.
	require.NoError(t, d.applyFlagToggle(ctx, 100, flagAutoScan))
	require.NoError(t, d.applyFlagToggle(ctx, 100, flagAutoScan))
	assert.Equal(t, scheduleCron, scheduleKindOf(d, 100))

	// Clearing it falls back to the service default.
	require.NoError(t, d.SetChannelSchedule(ctx, 100, nil, testAdminID))
	assert.Equal(t, scheduleInterval, scheduleKindOf(d, 100))
}

func TestSetChannelScheduleRejectsBadSchedule(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	defer d.StopCron()
	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{ID: 100, CommandChannelIDs: []int64{5}}))

	err := d.SetChannelSchedule(context.Background(), 100, &processors.ScanSchedule{At: "25:00"}, testAdminID)
	require.ErrorIs(t, err, ErrBadScheduleTime)

	stored, ok := storage.get(100)
	require.True(t, ok)
	assert.Nil(t, stored.ScanSchedule, "a rejected schedule must not be persisted")
}

func scheduleKindOf(d *Domain, channelID int64) scheduleKind {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	if schedule, ok := d.schedules[channelID]; ok {
		return schedule.kind
	}
	return -1
}

// TestDailyReportTimeReleasesOneSummaryPerDay: with a report time set, the
// summary is released by the first scan at or after it and the rest of the
// day's scans stay quiet - regardless of how many hours fall between them,
// which is what the IntervalHours throttle could not express.
//
// The rule is "at most one summary per day, released from At on", not "only
// between At and midnight": a scan before At still reports when nothing has
// gone out since the previous release. Withholding it would mean a channel
// only ever scanned before its report time never reported at all.
func TestDailyReportTimeReleasesOneSummaryPerDay(t *testing.T) {
	msk, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)

	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.scheduleTZ = "Europe/Moscow"
	clock := &clock{t: time.Date(2026, 11, 25, 8, 0, 0, 0, msk)}
	d.now = clock.now

	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5},
		Notifications: &processors.NotificationOptions{Mode: NotificationAll, IntervalHours: 24, At: "13:00"},
	}))

	report := func() bool {
		pc, _ := d.getProtectedChannel(100)
		r := processors.AccessReport{ReportChannels: []int64{5}}
		d.scheduledReport(context.Background(), pc, &r)
		return len(r.ReportChannels) > 0
	}

	// A channel that has never reported does so at once rather than waiting.
	assert.True(t, report(), "the first summary always goes out")

	// Yesterday's summary is out, and it is 08:00: today's is not due yet.
	require.NoError(t, d.updateProtectedChannel(context.Background(), 100, func(pc *ProtectedChannel) {
		pc.LastNotifiedAt = time.Date(2026, 11, 24, 13, 2, 0, 0, msk)
	}))
	assert.False(t, report(), "08:00 is before the 13:00 release time")

	// 13:05: the day's summary goes out.
	clock.t = time.Date(2026, 11, 25, 13, 5, 0, 0, msk)
	assert.True(t, report(), "the first scan after 13:00 reports")

	// 18:00 the same day: already reported.
	clock.t = time.Date(2026, 11, 25, 18, 0, 0, 0, msk)
	assert.False(t, report(), "only one summary per day")

	// Next day after the release time: a new summary.
	clock.t = time.Date(2026, 11, 26, 13, 1, 0, 0, msk)
	assert.True(t, report(), "a new day releases a new summary")
}

// Without a report time the IntervalHours throttle must behave exactly as
// before, so existing channels are unaffected.
func TestReportIntervalStillAppliesWithoutAReportTime(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	clock := &clock{t: time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)}
	d.now = clock.now

	require.NoError(t, d.AddProtectedChannel(&ProtectedChannel{
		ID: 100, CommandChannelIDs: []int64{5},
		Notifications: &processors.NotificationOptions{Mode: NotificationAll, IntervalHours: 6},
	}))

	report := func() bool {
		pc, _ := d.getProtectedChannel(100)
		r := processors.AccessReport{ReportChannels: []int64{5}}
		d.scheduledReport(context.Background(), pc, &r)
		return len(r.ReportChannels) > 0
	}

	assert.True(t, report(), "the first summary always goes out")
	clock.t = clock.t.Add(3 * time.Hour)
	assert.False(t, report(), "within the interval")
	clock.t = clock.t.Add(4 * time.Hour)
	assert.True(t, report(), "past the interval")
}
