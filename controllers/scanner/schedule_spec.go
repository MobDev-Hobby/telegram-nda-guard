package scanner

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// DefaultScheduleTZ is the zone a schedule is read in when neither the
// channel nor the service names one. A schedule without a zone would
// otherwise follow the host's local time, which in a container is whatever
// the image happens to set.
const DefaultScheduleTZ = "UTC"

var (
	// ErrBadScheduleTime is returned for a time of day that is not "HH:MM".
	ErrBadScheduleTime = errors.New("scan time must be HH:MM in 24-hour form")
	// ErrBadScheduleWeekday is returned for a weekday outside Sunday..Saturday.
	ErrBadScheduleWeekday = errors.New("weekday must be between 0 (Sunday) and 6 (Saturday)")
	// ErrBadScheduleTZ is returned for a zone the host cannot resolve.
	ErrBadScheduleTZ = errors.New("unknown time zone")
)

// parseTimeOfDay reads "HH:MM" into hour and minute.
func parseTimeOfDay(at string) (hour, minute int, err error) {
	h, m, found := strings.Cut(at, ":")
	if !found || len(h) != 2 || len(m) != 2 {
		return 0, 0, fmt.Errorf("%w: %q", ErrBadScheduleTime, at)
	}
	hour, err = strconv.Atoi(h)
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("%w: %q", ErrBadScheduleTime, at)
	}
	minute, err = strconv.Atoi(m)
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("%w: %q", ErrBadScheduleTime, at)
	}
	return hour, minute, nil
}

// scheduleLocation resolves the zone of a schedule, falling back to the
// service default and then to DefaultScheduleTZ.
func scheduleLocation(tz, fallback string) (*time.Location, error) {
	name := tz
	if name == "" {
		name = fallback
	}
	if name == "" {
		name = DefaultScheduleTZ
	}
	// A zone name reaches a cron spec as "CRON_TZ=<name> ...", so a name with
	// whitespace in it could smuggle in extra cron fields.
	if strings.ContainsAny(name, " \t") {
		return nil, fmt.Errorf("%w: %q", ErrBadScheduleTZ, name)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", ErrBadScheduleTZ, name)
	}
	return loc, nil
}

// validateSchedule reports whether a schedule can be compiled. fallbackTZ is
// the service default zone.
func validateSchedule(schedule processors.ScanSchedule, fallbackTZ string) error {
	_, err := cronSpec(schedule, fallbackTZ)
	return err
}

// cronSpec compiles a schedule into a cron expression carrying its own zone,
// e.g. "CRON_TZ=Europe/Moscow 5 13 * * 1,2,3,4,5". Per-entry zones let one
// scheduler serve channels in different zones.
func cronSpec(schedule processors.ScanSchedule, fallbackTZ string) (string, error) {
	hour, minute, err := parseTimeOfDay(schedule.At)
	if err != nil {
		return "", err
	}
	loc, err := scheduleLocation(schedule.TZ, fallbackTZ)
	if err != nil {
		return "", err
	}

	days := "*"
	if len(schedule.Weekdays) > 0 {
		unique := make([]int, 0, len(schedule.Weekdays))
		for _, day := range schedule.Weekdays {
			if day < time.Sunday || day > time.Saturday {
				return "", fmt.Errorf("%w: %d", ErrBadScheduleWeekday, int(day))
			}
			if !slices.Contains(unique, int(day)) {
				unique = append(unique, int(day))
			}
		}
		slices.Sort(unique)
		parts := make([]string, 0, len(unique))
		for _, day := range unique {
			parts = append(parts, strconv.Itoa(day))
		}
		days = strings.Join(parts, ",")
	}

	return fmt.Sprintf("CRON_TZ=%s %d %d * * %s", loc.String(), minute, hour, days), nil
}

// nextRun is when spec fires next after from. It exists so the service can log
// the resolved moment at startup instead of leaving operators to work out what
// a zone-qualified cron expression means.
func nextRun(spec string, from time.Time) (time.Time, error) {
	schedule, err := cron.ParseStandard(spec)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(from), nil
}

// reportBoundary is the most recent occurrence of the time of day at, in loc,
// at or before now. A summary is due when the last one went out before it, so
// exactly one summary is released per day from that time.
func reportBoundary(at string, loc *time.Location, now time.Time) (time.Time, error) {
	hour, minute, err := parseTimeOfDay(at)
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	boundary := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if boundary.After(local) {
		boundary = boundary.AddDate(0, 0, -1)
	}
	return boundary, nil
}

// weekdaysMonToFri is the working week, the schedule most channels want.
func weekdaysMonToFri() []time.Weekday {
	return []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday}
}
