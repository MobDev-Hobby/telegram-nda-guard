package scanner

import (
	"reflect"
	"time"
)

// scheduleKind records where a channel's scan schedule came from, so that
// rebuilding or dropping one never silently swaps a caller-supplied schedule
// (a cron, or a channel fed by the embedding application) for an interval
// ticker of the controller's own making.
type scheduleKind int

const (
	// scheduleInterval ticks every channelAutoScanInterval. The controller
	// owns the ticker and can stop it.
	scheduleInterval scheduleKind = iota
	// scheduleCustom is fed by the embedding application through
	// WithTickerChan/WithTicker/WithCron. Only its owner can stop it.
	scheduleCustom
)

// channelSchedule is the single scan/clean schedule of one protected channel.
//
// A schedule exists for as long as the channel is protected. It is NOT tied to
// the AutoScan/AutoClean flags: the ticker loop re-reads those flags on every
// tick, so a channel with automation switched off simply does nothing when its
// turn comes. Coupling the two is what made a channel lose its cron schedule
// for good once automation was switched off and on again — the schedule was
// deleted on the way down and replaced by a plain interval ticker on the way
// back up.
type channelSchedule struct {
	kind scheduleKind
	ch   <-chan time.Time
	// stop releases the underlying timer. It is nil for schedules the
	// controller does not own.
	stop func()
}

// newIntervalSchedule builds a schedule that ticks every interval. It returns
// nil for a non-positive interval, which leaves the channel without a
// schedule rather than panicking inside time.NewTicker.
func newIntervalSchedule(interval time.Duration) *channelSchedule {
	if interval <= 0 {
		return nil
	}
	ticker := time.NewTicker(interval)
	return &channelSchedule{kind: scheduleInterval, ch: ticker.C, stop: ticker.Stop}
}

// setSchedule installs the schedule of channelID, stopping the one it
// replaces. A channel has at most one schedule.
func (d *Domain) setSchedule(channelID int64, schedule *channelSchedule) {
	if schedule == nil {
		return
	}
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	d.stopScheduleLocked(channelID)
	if d.schedules == nil {
		// A Domain assembled without New() (tests, embedders) still gets a
		// working registry rather than a nil-map panic.
		d.schedules = make(map[int64]*channelSchedule)
	}
	d.schedules[channelID] = schedule
}

// ensureIntervalSchedule gives channelID the default interval schedule unless
// it already has one, so a channel never ends up unscheduled. It never
// replaces a custom schedule.
func (d *Domain) ensureIntervalSchedule(channelID int64) {
	d.scheduleMutex.Lock()
	if _, ok := d.schedules[channelID]; ok {
		d.scheduleMutex.Unlock()
		return
	}
	d.scheduleMutex.Unlock()

	if schedule := newIntervalSchedule(d.channelAutoScanInterval); schedule != nil {
		d.setSchedule(channelID, schedule)
	}
}

// dropSchedule removes the schedule of channelID. It is called when a channel
// stops being protected, never when its automation flags change.
func (d *Domain) dropSchedule(channelID int64) {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	d.stopScheduleLocked(channelID)
	delete(d.schedules, channelID)
}

// migrateSchedule moves a schedule to the id Telegram migrated the chat to,
// keeping its kind and underlying timer.
func (d *Domain) migrateSchedule(fromChannelID, toChannelID int64) {
	if fromChannelID == toChannelID {
		return
	}
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	schedule, ok := d.schedules[fromChannelID]
	if !ok {
		return
	}
	d.stopScheduleLocked(toChannelID)
	if d.schedules == nil {
		d.schedules = make(map[int64]*channelSchedule)
	}
	d.schedules[toChannelID] = schedule
	delete(d.schedules, fromChannelID)
}

// hasSchedule reports whether channelID is scheduled.
func (d *Domain) hasSchedule(channelID int64) bool {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	_, ok := d.schedules[channelID]
	return ok
}

// stopScheduleLocked stops the schedule of channelID, if any, without removing
// it from the map. Callers must hold scheduleMutex.
func (d *Domain) stopScheduleLocked(channelID int64) {
	if schedule, ok := d.schedules[channelID]; ok && schedule.stop != nil {
		schedule.stop()
	}
}

// scheduleCases snapshots the registry for reflect.Select. Cases and ids are
// built in one pass, so the two can never drift apart the way the previous
// pair of parallel slices could.
func (d *Domain) scheduleCases() ([]reflect.SelectCase, []int64) {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	cases := make([]reflect.SelectCase, 0, len(d.schedules)+1)
	ids := make([]int64, 0, len(d.schedules))
	for channelID, schedule := range d.schedules {
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(schedule.ch)})
		ids = append(ids, channelID)
	}
	return cases, ids
}

// stopSchedules stops every schedule the controller owns. It is used on
// shutdown so interval tickers do not outlive the domain.
func (d *Domain) stopSchedules() {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	for channelID := range d.schedules {
		d.stopScheduleLocked(channelID)
		delete(d.schedules, channelID)
	}
}
