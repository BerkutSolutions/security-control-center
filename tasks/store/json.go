package store

import (
	"encoding/json"
	"strings"
)

func marshalJSON(v any) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func normalizeJSON(raw string, fallback string) string {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return fallback
	}
	var tmp any
	if err := json.Unmarshal([]byte(clean), &tmp); err != nil {
		return fallback
	}
	return clean
}

func unmarshalJSON(raw string, target any) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return
	}
	_ = json.Unmarshal([]byte(clean), target)
}
