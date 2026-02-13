package backups

import (
	"strconv"
	"time"
)

const (
	ScheduleDaily        = "daily"
	ScheduleWeekly       = "weekly"
	ScheduleMonthlyStart = "monthly_start"
	ScheduleMonthlyEnd   = "monthly_end"
	MonthAnchorStart     = "start"
	MonthAnchorEnd       = "end"
)

func nextRunAfter(plan BackupPlan, ref time.Time) time.Time {
	loc := ref.Location()
	if loc == nil {
		loc = time.UTC
	}
	hour := clamp(plan.ScheduleHour, 0, 23)
	minute := clamp(plan.ScheduleMinute, 0, 59)
	base := ref.In(loc)

	switch plan.ScheduleType {
	case ScheduleWeekly:
		weekday := time.Weekday(clamp(plan.ScheduleWeekday, 0, 6))
		candidate := time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, loc)
		delta := (int(weekday) - int(candidate.Weekday()) + 7) % 7
		candidate = candidate.AddDate(0, 0, delta)
		if !candidate.After(base) {
			candidate = candidate.AddDate(0, 0, 7)
		}
		return candidate.UTC()
	case ScheduleMonthlyStart:
		candidate := time.Date(base.Year(), base.Month(), 1, hour, minute, 0, 0, loc)
		if !candidate.After(base) {
			candidate = time.Date(base.Year(), base.Month()+1, 1, hour, minute, 0, 0, loc)
		}
		return candidate.UTC()
	case ScheduleMonthlyEnd:
		candidate := time.Date(base.Year(), base.Month()+1, 0, hour, minute, 0, 0, loc)
		if !candidate.After(base) {
			candidate = time.Date(base.Year(), base.Month()+2, 0, hour, minute, 0, 0, loc)
		}
		return candidate.UTC()
	default:
		candidate := time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, loc)
		if !candidate.After(base) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate.UTC()
	}
}

func scheduleCronExpression(plan BackupPlan) string {
	hour := clamp(plan.ScheduleHour, 0, 23)
	minute := clamp(plan.ScheduleMinute, 0, 59)
	switch plan.ScheduleType {
	case ScheduleWeekly:
		return itoa(minute) + " " + itoa(hour) + " * * " + itoa(clamp(plan.ScheduleWeekday, 0, 6))
	case ScheduleMonthlyStart:
		return itoa(minute) + " " + itoa(hour) + " 1 * *"
	case ScheduleMonthlyEnd:
		return itoa(minute) + " " + itoa(hour) + " 28-31 * *"
	default:
		return itoa(minute) + " " + itoa(hour) + " * * *"
	}
}

func shouldRunByPlan(plan BackupPlan, lastRun *time.Time, now time.Time) bool {
	reference := now.UTC()
	if lastRun != nil {
		reference = lastRun.UTC()
	}
	next := nextRunAfter(plan, reference)
	if !next.After(now.UTC()) {
		if plan.ScheduleType == ScheduleMonthlyEnd {
			n := now.UTC()
			lastDay := time.Date(n.Year(), n.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			return n.Day() == lastDay
		}
		return true
	}
	return false
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
