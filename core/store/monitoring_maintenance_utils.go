package store

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

func normalizeMonitorIDs(in []int64) []int64 {
	if len(in) == 0 {
		return nil
	}
	set := map[int64]struct{}{}
	out := make([]int64, 0, len(in))
	for _, id := range in {
		if id <= 0 {
			continue
		}
		if _, exists := set[id]; exists {
			continue
		}
		set[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeWeekdays(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	set := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, d := range in {
		if d < 1 || d > 7 {
			continue
		}
		if _, exists := set[d]; exists {
			continue
		}
		set[d] = struct{}{}
		out = append(out, d)
	}
	sort.Ints(out)
	return out
}

func normalizeMonthDays(in []int) []int {
	if len(in) == 0 {
		return nil
	}
	set := map[int]struct{}{}
	out := make([]int, 0, len(in))
	for _, d := range in {
		if d < 1 || d > 31 {
			continue
		}
		if _, exists := set[d]; exists {
			continue
		}
		set[d] = struct{}{}
		out = append(out, d)
	}
	sort.Ints(out)
	return out
}

func normalizeHHMM(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return ""
	}
	h, errH := toInt(strings.TrimSpace(parts[0]))
	m, errM := toInt(strings.TrimSpace(parts[1]))
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return ""
	}
	return twoDigit(h) + ":" + twoDigit(m)
}

func int64SliceToJSON(in []int64) string {
	if in == nil {
		in = []int64{}
	}
	b, _ := json.Marshal(in)
	return string(b)
}

func maintenanceScheduleToJSON(in MaintenanceSchedule) string {
	b, _ := json.Marshal(in)
	return string(b)
}

func mergeMaintenanceWindows(in []MaintenanceWindow) []MaintenanceWindow {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		return in[i].Start.Before(in[j].Start)
	})
	out := make([]MaintenanceWindow, 0, len(in))
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

func boolPtr(v bool) *bool {
	return &v
}

func toInt(raw string) (int, error) {
	return strconv.Atoi(raw)
}

func twoDigit(v int) string {
	if v < 10 {
		return "0" + intToString(v)
	}
	return intToString(v)
}
