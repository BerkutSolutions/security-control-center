package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ScheduleDaily     = "daily"
	ScheduleWeekly    = "weekly"
	ScheduleMonthly   = "monthly"
	ScheduleQuarterly = "quarterly"
	ScheduleSemiAnnual = "semiannual"
	ScheduleAnnual    = "annual"
)

type WeeklyScheduleConfig struct {
	Weekdays []int `json:"weekdays"`
}

type MonthlyScheduleConfig struct {
	Day int `json:"day"`
}

type MonthDayScheduleConfig struct {
	Month int `json:"month"`
	Day   int `json:"day"`
}

func NormalizeScheduleConfig(scheduleType string, raw json.RawMessage) (json.RawMessage, error) {
	switch strings.ToLower(strings.TrimSpace(scheduleType)) {
	case ScheduleDaily:
		return json.RawMessage(`{}`), nil
	case ScheduleWeekly:
		var cfg WeeklyScheduleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, errors.New("tasks.recurring.weekdaysRequired")
		}
		unique := map[int]struct{}{}
		for _, d := range cfg.Weekdays {
			if d < 0 || d > 6 {
				return nil, errors.New("tasks.recurring.weekdaysRequired")
			}
			unique[d] = struct{}{}
		}
		if len(unique) == 0 {
			return nil, errors.New("tasks.recurring.weekdaysRequired")
		}
		cfg.Weekdays = cfg.Weekdays[:0]
		for d := range unique {
			cfg.Weekdays = append(cfg.Weekdays, d)
		}
		sort.Ints(cfg.Weekdays)
		out, _ := json.Marshal(cfg)
		return out, nil
	case ScheduleMonthly:
		var cfg MonthlyScheduleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, errors.New("tasks.recurring.dayRequired")
		}
		if cfg.Day < 1 || cfg.Day > 31 {
			return nil, errors.New("tasks.recurring.dayRequired")
		}
		out, _ := json.Marshal(cfg)
		return out, nil
	case ScheduleQuarterly, ScheduleSemiAnnual, ScheduleAnnual:
		var cfg MonthDayScheduleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, errors.New("tasks.recurring.monthDayRequired")
		}
		if cfg.Month < 1 || cfg.Month > 12 || cfg.Day < 1 || cfg.Day > 31 {
			return nil, errors.New("tasks.recurring.monthDayRequired")
		}
		out, _ := json.Marshal(cfg)
		return out, nil
	default:
		return nil, errors.New("tasks.recurring.typeInvalid")
	}
}

func ComputeNextRunAt(base time.Time, scheduleType string, raw json.RawMessage, timeOfDay string) (time.Time, error) {
	hour, min, err := parseTimeOfDay(timeOfDay)
	if err != nil {
		return time.Time{}, err
	}
	base = base.UTC()
	switch strings.ToLower(strings.TrimSpace(scheduleType)) {
	case ScheduleDaily:
		return nextDaily(base, hour, min), nil
	case ScheduleWeekly:
		var cfg WeeklyScheduleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return time.Time{}, errors.New("tasks.recurring.weekdaysRequired")
		}
		return nextWeekly(base, cfg.Weekdays, hour, min)
	case ScheduleMonthly:
		var cfg MonthlyScheduleConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return time.Time{}, errors.New("tasks.recurring.dayRequired")
		}
		return nextMonthly(base, cfg.Day, hour, min), nil
	case ScheduleQuarterly:
		return nextByInterval(base, raw, hour, min, 3)
	case ScheduleSemiAnnual:
		return nextByInterval(base, raw, hour, min, 6)
	case ScheduleAnnual:
		return nextByInterval(base, raw, hour, min, 12)
	default:
		return time.Time{}, errors.New("tasks.recurring.typeInvalid")
	}
}

func parseTimeOfDay(val string) (int, int, error) {
	clean := strings.TrimSpace(val)
	if clean == "" {
		return 0, 0, errors.New("tasks.recurring.timeRequired")
	}
	parts := strings.Split(clean, ":")
	if len(parts) != 2 {
		return 0, 0, errors.New("tasks.recurring.timeRequired")
	}
	var hour, min int
	if _, err := fmt.Sscanf(clean, "%d:%d", &hour, &min); err != nil {
		return 0, 0, errors.New("tasks.recurring.timeRequired")
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, 0, errors.New("tasks.recurring.timeRequired")
	}
	return hour, min, nil
}

func nextDaily(base time.Time, hour, min int) time.Time {
	candidate := time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, base.Location())
	if !candidate.After(base) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

func nextWeekly(base time.Time, weekdays []int, hour, min int) (time.Time, error) {
	if len(weekdays) == 0 {
		return time.Time{}, errors.New("tasks.recurring.weekdaysRequired")
	}
	allowed := map[time.Weekday]struct{}{}
	for _, d := range weekdays {
		if d < 0 || d > 6 {
			return time.Time{}, errors.New("tasks.recurring.weekdaysRequired")
		}
		allowed[time.Weekday(d)] = struct{}{}
	}
	for i := 0; i < 7; i++ {
		date := base.AddDate(0, 0, i)
		if _, ok := allowed[date.Weekday()]; !ok {
			continue
		}
		candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, base.Location())
		if candidate.After(base) {
			return candidate, nil
		}
	}
	return time.Time{}, errors.New("tasks.recurring.weekdaysRequired")
}

func nextMonthly(base time.Time, day, hour, min int) time.Time {
	if day < 1 {
		day = 1
	}
	candidate := time.Date(base.Year(), base.Month(), clampDay(base.Year(), base.Month(), day), hour, min, 0, 0, base.Location())
	if !candidate.After(base) {
		candidate = addMonths(candidate, 1, day, hour, min)
	}
	return candidate
}

func nextByInterval(base time.Time, raw json.RawMessage, hour, min, interval int) (time.Time, error) {
	var cfg MonthDayScheduleConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return time.Time{}, errors.New("tasks.recurring.monthDayRequired")
	}
	if cfg.Month < 1 || cfg.Month > 12 || cfg.Day < 1 || cfg.Day > 31 {
		return time.Time{}, errors.New("tasks.recurring.monthDayRequired")
	}
	year := base.Year()
	candidate := time.Date(year, time.Month(cfg.Month), clampDay(year, time.Month(cfg.Month), cfg.Day), hour, min, 0, 0, base.Location())
	for !candidate.After(base) {
		candidate = addMonths(candidate, interval, cfg.Day, hour, min)
	}
	return candidate, nil
}

func addMonths(base time.Time, months, day, hour, min int) time.Time {
	next := base.AddDate(0, months, 0)
	return time.Date(next.Year(), next.Month(), clampDay(next.Year(), next.Month(), day), hour, min, 0, 0, base.Location())
}

func clampDay(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		return last
	}
	if day < 1 {
		return 1
	}
	return day
}
