package scanner

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
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
	// scheduleCron fires at a time of day in a given zone. The controller
	// owns the entry and can remove it.
	scheduleCron
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
	d.wakeTickerLocked()
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
	d.wakeTickerLocked()
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
	d.wakeTickerLocked()
}

// hasSchedule reports whether channelID is scheduled.
func (d *Domain) hasSchedule(channelID int64) bool {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	_, ok := d.schedules[channelID]
	return ok
}

// wakeTickerLocked tells the ticker loop that the registry changed, so it
// re-snapshots instead of staying parked in reflect.Select on schedules that
// may no longer exist. Callers must hold scheduleMutex; the send never blocks.
func (d *Domain) wakeTickerLocked() {
	if d.scheduleChanged == nil {
		d.scheduleChanged = make(chan struct{}, 1)
	}
	select {
	case d.scheduleChanged <- struct{}{}:
	default:
		// A wake-up is already pending; the loop will re-snapshot once and
		// see every change made in the meantime.
	}
}

// tickerWakeup returns the channel the ticker loop selects on for registry
// changes.
func (d *Domain) tickerWakeup() <-chan struct{} {
	d.scheduleMutex.Lock()
	defer d.scheduleMutex.Unlock()
	if d.scheduleChanged == nil {
		d.scheduleChanged = make(chan struct{}, 1)
	}
	return d.scheduleChanged
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

// cronRunner returns the one scheduler shared by every cron-scheduled channel,
// starting it on first use. One scheduler with a zone-qualified entry per
// channel replaces a scheduler per channel, each of which used to be started
// and then leaked.
func (d *Domain) cronRunner() *cron.Cron {
	d.cronMutex.Lock()
	defer d.cronMutex.Unlock()
	if d.cron == nil {
		d.cron = cron.New()
		d.cron.Start()
	}
	return d.cron
}

// StopCron stops the shared scheduler. Entries already running finish.
func (d *Domain) StopCron() {
	d.cronMutex.Lock()
	runner := d.cron
	d.cron = nil
	d.cronMutex.Unlock()
	if runner != nil {
		runner.Stop()
	}
}

// newCronSchedule builds a schedule that fires on spec.
func (d *Domain) newCronSchedule(spec string) (*channelSchedule, error) {
	ticks := make(chan time.Time, 1)
	runner := d.cronRunner()
	entryID, err := runner.AddFunc(spec, func() {
		select {
		case ticks <- time.Now():
		default:
			// A tick is already pending; the channel has not been scanned
			// since. One pending tick is enough.
		}
	})
	if err != nil {
		return nil, fmt.Errorf("schedule %q: %w", spec, err)
	}
	return &channelSchedule{
		kind: scheduleCron,
		ch:   ticks,
		stop: func() { runner.Remove(entryID) },
	}, nil
}

// effectiveSchedule resolves the schedule that applies to a channel from a
// record the caller already holds: the channel's own when it has one,
// otherwise the service default, otherwise nil for channels that fall back to
// the interval ticker. custom reports which of the two it is.
//
// It takes no locks, so it is safe to call while holding channelsMutex.
// defaultScanSchedule is only written while the Domain is being configured.
func (d *Domain) effectiveSchedule(pc ProtectedChannel) (schedule *processors.ScanSchedule, custom bool) {
	if pc.ScanSchedule != nil {
		return pc.ScanSchedule, true
	}
	return d.defaultScanSchedule, false
}

// channelScanSchedule is effectiveSchedule for callers that do not hold
// channelsMutex.
func (d *Domain) channelScanSchedule(channelID int64) *processors.ScanSchedule {
	d.channelsMutex.RLock()
	pc := d.protectedChannels[channelID]
	d.channelsMutex.RUnlock()
	schedule, _ := d.effectiveSchedule(pc)
	return schedule
}

// installSchedule gives channelID the schedule that applies to it. A schedule
// supplied by the embedding application (WithTickerChan and friends) is left
// alone: only its owner manages it.
func (d *Domain) installSchedule(channelID int64) error {
	d.scheduleMutex.Lock()
	existing, ok := d.schedules[channelID]
	d.scheduleMutex.Unlock()
	if ok && existing.kind == scheduleCustom {
		return nil
	}
	return d.reinstallSchedule(channelID)
}

// reinstallSchedule replaces channelID's schedule with the one that applies to
// it now: a cron entry when a schedule is configured for the channel or the
// service, otherwise the interval ticker. It is what an explicit schedule
// change goes through.
func (d *Domain) reinstallSchedule(channelID int64) error {
	schedule := d.channelScanSchedule(channelID)
	if schedule == nil {
		interval := newIntervalSchedule(d.channelAutoScanInterval)
		if interval == nil {
			return nil
		}
		d.setSchedule(channelID, interval)
		return nil
	}
	spec, err := cronSpec(*schedule, d.scheduleTZ)
	if err != nil {
		return fmt.Errorf("channel %d: %w", channelID, err)
	}
	cronSchedule, err := d.newCronSchedule(spec)
	if err != nil {
		return fmt.Errorf("channel %d: %w", channelID, err)
	}
	d.setSchedule(channelID, cronSchedule)
	if next, nextErr := nextRun(spec, time.Now()); nextErr == nil {
		d.log.Infof("channel %d scans on %q, next run %s", channelID, spec, next.Format(time.RFC3339))
	}
	return nil
}

// scheduleLocationOf resolves the zone a channel's times are read in, for the
// daily summary as well as the scan itself. It takes no locks.
func (d *Domain) scheduleLocationOf(pc ProtectedChannel) *time.Location {
	tz := ""
	if schedule, _ := d.effectiveSchedule(pc); schedule != nil {
		tz = schedule.TZ
	}
	loc, err := scheduleLocation(tz, d.scheduleTZ)
	if err != nil {
		d.log.Warnf("channel %d: %s, falling back to UTC", pc.ID, err)
		return time.UTC
	}
	return loc
}

// describeScheduleOf renders a channel's effective schedule for an operator,
// naming the zone so a bare time is never ambiguous. It takes no locks.
func (d *Domain) describeScheduleOf(pc ProtectedChannel) string {
	schedule, _ := d.effectiveSchedule(pc)
	if schedule == nil {
		return fmt.Sprintf("every %s", d.channelAutoScanInterval)
	}
	days := "every day"
	if len(schedule.Weekdays) > 0 {
		names := make([]string, 0, len(schedule.Weekdays))
		for _, day := range schedule.Weekdays {
			names = append(names, day.String())
		}
		days = strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, %s (%s)", schedule.At, days, d.scheduleLocationOf(pc))
}
