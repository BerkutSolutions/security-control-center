package store

import "time"

// boolToInt converts a boolean into 0/1 for SQLite booleans.
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableTime(ts *time.Time) any {
	if ts == nil {
		return nil
	}
	return *ts
}
