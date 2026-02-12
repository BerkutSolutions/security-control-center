package charts

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseInt(val string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(val))
}

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func getTime(m map[string]any, key string) (time.Time, bool) {
	raw := getString(m, key)
	if raw == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), true
	}
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

func getDate(m map[string]any, key string) (time.Time, bool) {
	raw := getString(m, key)
	if raw == "" || raw == "-" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse("2006-01-02", raw); err == nil {
		return parsed.UTC(), true
	}
	return time.Time{}, false
}

func getFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	val, ok := m[key]
	if !ok {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	}
	return 0
}

func getInt(m map[string]any, key string) *int {
	if m == nil {
		return nil
	}
	val, ok := m[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case float64:
		iv := int(v)
		return &iv
	case int:
		iv := v
		return &iv
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return &parsed
		}
	}
	return nil
}

func getBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	val, ok := m[key]
	if !ok {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.TrimSpace(strings.ToLower(v)) == "true"
	default:
		return false
	}
}
