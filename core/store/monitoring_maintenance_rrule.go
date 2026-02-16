package store

import (
	"errors"
	"strings"
	"time"
)

type rruleSpec struct {
	freq     string
	interval int
	byDay    []time.Weekday
}

func parseRRule(raw string) (*rruleSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errInvalidRRule
	}
	parts := strings.Split(raw, ";")
	spec := &rruleSpec{interval: 1}
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, errInvalidRRule
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		val := strings.ToUpper(strings.TrimSpace(kv[1]))
		switch key {
		case "FREQ":
			spec.freq = val
		case "INTERVAL":
			n, err := toInt(val)
			if err != nil || n <= 0 {
				return nil, errInvalidRRule
			}
			spec.interval = n
		case "BYDAY":
			if val == "" {
				continue
			}
			days := strings.Split(val, ",")
			for _, d := range days {
				if wd, ok := parseWeekday(d); ok {
					spec.byDay = append(spec.byDay, wd)
				}
			}
		default:
			return nil, errInvalidRRule
		}
	}
	if spec.freq != "DAILY" && spec.freq != "WEEKLY" {
		return nil, errInvalidRRule
	}
	if spec.freq == "WEEKLY" && len(spec.byDay) == 0 {
		spec.byDay = []time.Weekday{time.Monday}
	}
	return spec, nil
}

func rruleMatchesDay(spec *rruleSpec, startDay, day time.Time) bool {
	if spec == nil {
		return false
	}
	days := int(dateOnly(day).Sub(dateOnly(startDay)).Hours() / 24)
	if days < 0 {
		return false
	}
	switch spec.freq {
	case "DAILY":
		return days%spec.interval == 0
	case "WEEKLY":
		weeks := days / 7
		if weeks%spec.interval != 0 {
			return false
		}
		for _, wd := range spec.byDay {
			if wd == day.Weekday() {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func parseWeekday(raw string) (time.Weekday, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MO":
		return time.Monday, true
	case "TU":
		return time.Tuesday, true
	case "WE":
		return time.Wednesday, true
	case "TH":
		return time.Thursday, true
	case "FR":
		return time.Friday, true
	case "SA":
		return time.Saturday, true
	case "SU":
		return time.Sunday, true
	default:
		return time.Sunday, false
	}
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

var errInvalidRRule = errors.New("invalid rrule")
