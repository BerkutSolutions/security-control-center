package store

import (
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	maintenanceStrategySingle   = "single"
	maintenanceStrategyCron     = "cron"
	maintenanceStrategyInterval = "interval"
	maintenanceStrategyWeekday  = "weekday"
	maintenanceStrategyMonthday = "monthday"
	maintenanceStrategyRRule    = "rrule"
)

type maintenanceRange struct {
	Start time.Time
	End   time.Time
}

func maintenanceActiveAt(m MonitorMaintenance, now time.Time) bool {
	ranges := maintenanceWindowsWithin(m, now.Add(-time.Minute), now.Add(time.Minute))
	for _, rng := range ranges {
		if (now.After(rng.Start) || now.Equal(rng.Start)) && now.Before(rng.End) {
			return true
		}
	}
	return false
}

func maintenanceWindowsWithin(m MonitorMaintenance, since, until time.Time) []maintenanceRange {
	if !until.After(since) || !m.IsActive {
		return nil
	}
	strategy := strings.ToLower(strings.TrimSpace(m.Strategy))
	if strategy == "" {
		if m.IsRecurring {
			strategy = maintenanceStrategyRRule
		} else {
			strategy = maintenanceStrategySingle
		}
	}
	switch strategy {
	case maintenanceStrategySingle:
		return overlapSingle(m.StartsAt.UTC(), m.EndsAt.UTC(), since.UTC(), until.UTC())
	case maintenanceStrategyCron:
		return windowsForCron(m, since.UTC(), until.UTC())
	case maintenanceStrategyInterval:
		return windowsForDailyStrategy(m, since.UTC(), until.UTC(), func(day time.Time, anchor time.Time) bool {
			interval := m.Schedule.IntervalDays
			if interval <= 0 {
				interval = 1
			}
			days := int(day.Sub(dateOnly(anchor)).Hours() / 24)
			return days >= 0 && days%interval == 0
		})
	case maintenanceStrategyWeekday:
		allowed := map[time.Weekday]struct{}{}
		for _, day := range m.Schedule.Weekdays {
			wd := scheduleWeekdayToGo(day)
			if wd >= time.Sunday && wd <= time.Saturday {
				allowed[wd] = struct{}{}
			}
		}
		if len(allowed) == 0 {
			return nil
		}
		return windowsForDailyStrategy(m, since.UTC(), until.UTC(), func(day time.Time, _ time.Time) bool {
			_, ok := allowed[day.Weekday()]
			return ok
		})
	case maintenanceStrategyMonthday:
		allowed := map[int]struct{}{}
		for _, day := range m.Schedule.MonthDays {
			if day >= 1 && day <= 31 {
				allowed[day] = struct{}{}
			}
		}
		useLastDay := m.Schedule.UseLastDay
		return windowsForDailyStrategy(m, since.UTC(), until.UTC(), func(day time.Time, _ time.Time) bool {
			if _, ok := allowed[day.Day()]; ok {
				return true
			}
			if !useLastDay {
				return false
			}
			last := time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, day.Location())
			return day.Day() == last.Day()
		})
	case maintenanceStrategyRRule:
		return windowsForLegacyRRule(m, since.UTC(), until.UTC())
	default:
		return overlapSingle(m.StartsAt.UTC(), m.EndsAt.UTC(), since.UTC(), until.UTC())
	}
}

func windowsForCron(m MonitorMaintenance, since, until time.Time) []maintenanceRange {
	if strings.TrimSpace(m.Schedule.CronExpression) == "" || m.Schedule.DurationMin <= 0 {
		return nil
	}
	loc := maintenanceLocation(m.Timezone)
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sch, err := parser.Parse(strings.TrimSpace(m.Schedule.CronExpression))
	if err != nil {
		return nil
	}
	startBound, endBound := maintenanceBounds(m, loc)
	duration := time.Duration(m.Schedule.DurationMin) * time.Minute
	reference := since.In(loc).Add(-duration)
	if !startBound.IsZero() && startBound.After(reference) {
		reference = startBound.Add(-time.Minute)
	}
	limit := until.In(loc)
	var out []maintenanceRange
	for i := 0; i < 20000; i++ {
		next := sch.Next(reference)
		if !next.Before(limit.Add(time.Minute)) {
			break
		}
		reference = next
		if !startBound.IsZero() && next.Before(startBound) {
			continue
		}
		if !endBound.IsZero() && !next.Before(endBound) {
			break
		}
		startUTC := next.UTC()
		endUTC := next.Add(duration).UTC()
		if !endBound.IsZero() && endUTC.After(endBound.UTC()) {
			endUTC = endBound.UTC()
		}
		out = append(out, overlapSingle(startUTC, endUTC, since, until)...)
	}
	return mergeMaintenanceRanges(out)
}

func windowsForDailyStrategy(m MonitorMaintenance, since, until time.Time, acceptDay func(day time.Time, anchor time.Time) bool) []maintenanceRange {
	if m.Schedule.WindowStart == "" || m.Schedule.WindowEnd == "" {
		return nil
	}
	loc := maintenanceLocation(m.Timezone)
	startH, startM, okStart := parseHHMM(m.Schedule.WindowStart)
	endH, endM, okEnd := parseHHMM(m.Schedule.WindowEnd)
	if !okStart || !okEnd {
		return nil
	}
	startBound, endBound := maintenanceBounds(m, loc)
	anchor := m.StartsAt.In(loc)
	if m.Schedule.ActiveFrom != nil {
		anchor = m.Schedule.ActiveFrom.In(loc)
	}
	cursor := dateOnly(since.In(loc).Add(-24 * time.Hour))
	last := dateOnly(until.In(loc).Add(24 * time.Hour))
	var out []maintenanceRange
	for !cursor.After(last) {
		if !startBound.IsZero() && cursor.Add(24*time.Hour).Before(startBound) {
			cursor = cursor.Add(24 * time.Hour)
			continue
		}
		if !endBound.IsZero() && !cursor.Before(endBound.Add(24*time.Hour)) {
			break
		}
		if acceptDay(cursor, anchor) {
			startLocal := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), startH, startM, 0, 0, loc)
			endLocal := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), endH, endM, 0, 0, loc)
			if !endLocal.After(startLocal) {
				endLocal = endLocal.Add(24 * time.Hour)
			}
			if !startBound.IsZero() && startLocal.Before(startBound) {
				startLocal = startBound
			}
			if !endBound.IsZero() && endLocal.After(endBound) {
				endLocal = endBound
			}
			out = append(out, overlapSingle(startLocal.UTC(), endLocal.UTC(), since, until)...)
		}
		cursor = cursor.Add(24 * time.Hour)
	}
	return mergeMaintenanceRanges(out)
}

func windowsForLegacyRRule(m MonitorMaintenance, since, until time.Time) []maintenanceRange {
	if !m.IsRecurring {
		return overlapSingle(m.StartsAt.UTC(), m.EndsAt.UTC(), since.UTC(), until.UTC())
	}
	spec, err := parseRRule(m.RRuleText)
	if err != nil {
		return nil
	}
	loc := maintenanceLocation(m.Timezone)
	duration := m.EndsAt.Sub(m.StartsAt)
	if duration <= 0 {
		duration = time.Hour
	}
	startDay := dateOnly(m.StartsAt.In(loc))
	cursor := dateOnly(since.In(loc).Add(-24 * time.Hour))
	last := dateOnly(until.In(loc).Add(24 * time.Hour))
	var out []maintenanceRange
	for !cursor.After(last) {
		if rruleMatchesDay(spec, startDay, cursor) {
			startLocal := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), m.StartsAt.In(loc).Hour(), m.StartsAt.In(loc).Minute(), m.StartsAt.In(loc).Second(), 0, loc)
			endLocal := startLocal.Add(duration)
			out = append(out, overlapSingle(startLocal.UTC(), endLocal.UTC(), since.UTC(), until.UTC())...)
		}
		cursor = cursor.Add(24 * time.Hour)
	}
	return mergeMaintenanceRanges(out)
}

func overlapSingle(start, end, since, until time.Time) []maintenanceRange {
	if !end.After(start) {
		return nil
	}
	if end.Before(since) || start.After(until) || start.Equal(until) {
		return nil
	}
	if start.Before(since) {
		start = since
	}
	if end.After(until) {
		end = until
	}
	if !end.After(start) {
		return nil
	}
	return []maintenanceRange{{Start: start, End: end}}
}

func mergeMaintenanceRanges(in []maintenanceRange) []maintenanceRange {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].Start.Before(in[j].Start)
	})
	out := make([]maintenanceRange, 0, len(in))
	for _, item := range in {
		if !item.End.After(item.Start) {
			continue
		}
		if len(out) == 0 {
			out = append(out, item)
			continue
		}
		last := &out[len(out)-1]
		if item.Start.After(last.End) {
			out = append(out, item)
			continue
		}
		if item.End.After(last.End) {
			last.End = item.End
		}
	}
	return out
}

func maintenanceBounds(m MonitorMaintenance, loc *time.Location) (time.Time, time.Time) {
	var startBound time.Time
	var endBound time.Time
	if m.Schedule.ActiveFrom != nil {
		startBound = m.Schedule.ActiveFrom.In(loc)
	} else if !m.StartsAt.IsZero() {
		startBound = m.StartsAt.In(loc)
	}
	if m.Schedule.ActiveUntil != nil {
		endBound = m.Schedule.ActiveUntil.In(loc)
	}
	return startBound, endBound
}

func maintenanceLocation(raw string) *time.Location {
	name := strings.TrimSpace(raw)
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseHHMM(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hh := strings.TrimSpace(parts[0])
	mm := strings.TrimSpace(parts[1])
	if len(hh) == 1 {
		hh = "0" + hh
	}
	if len(mm) == 1 {
		mm = "0" + mm
	}
	parsed, err := time.Parse("15:04", hh+":"+mm)
	if err != nil {
		return 0, 0, false
	}
	return parsed.Hour(), parsed.Minute(), true
}

func scheduleWeekdayToGo(day int) time.Weekday {
	switch day {
	case 1:
		return time.Monday
	case 2:
		return time.Tuesday
	case 3:
		return time.Wednesday
	case 4:
		return time.Thursday
	case 5:
		return time.Friday
	case 6:
		return time.Saturday
	case 7:
		return time.Sunday
	default:
		return time.Sunday
	}
}
