package cli

import (
	"fmt"
	"strings"
	"time"
)

func splitRoles(r string) []string {
	var res []string
	for _, part := range strings.Split(r, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			res = append(res, part)
		}
	}
	return res
}

func parseSince(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("expected YYYY-MM-DD or RFC3339")
}

