package experiment

import (
	"strings"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
)

func classifyRawStatus(o Observation) (string, monitoring.ErrorKind) {
	res := monitoring.CheckResult{
		OK:         o.OK,
		LatencyMs:  o.LatencyMs,
		StatusCode: o.StatusCode,
		Error:      strings.TrimSpace(o.Error),
		CheckedAt:  o.TS,
	}
	kind := monitoring.ClassifyResultKind(res)
	if res.OK {
		return "up", kind
	}
	if kind == monitoring.ErrorKindDNS {
		return "dns", kind
	}
	return "down", kind
}

func simulateState(prev *store.MonitorState, now time.Time, raw string, kind monitoring.ErrorKind, o Observation) *store.MonitorState {
	next := &store.MonitorState{
		MonitorID:        0,
		Status:           raw,
		LastResultStatus: raw,
		LastErrorKind:    string(kind),
		LastError:        strings.TrimSpace(o.Error),
		LastCheckedAt:    &now,
	}
	if o.StatusCode != nil {
		val := *o.StatusCode
		next.LastStatusCode = &val
	}
	if o.LatencyMs > 0 {
		val := o.LatencyMs
		next.LastLatencyMs = &val
	}
	if prev != nil {
		next.LastUpAt = prev.LastUpAt
		next.LastDownAt = prev.LastDownAt
	}
	if raw == "up" {
		next.LastUpAt = &now
	} else {
		// Keep outage start (best-effort) for scoring.
		if prev == nil || strings.ToLower(strings.TrimSpace(prev.LastResultStatus)) == "up" || prev.LastDownAt == nil {
			next.LastDownAt = &now
		}
	}
	return next
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
