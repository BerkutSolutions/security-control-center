package monitoring

import (
	"math"
	"strings"
	"time"
)

// ComputeIncidentScore implements a simple, explainable heuristic scoring model.
// It is deterministic and depends only on the provided input.
//
// The intent is to provide a stable foundation for later stages (DB/API/UI integration and experiments),
// not to be a perfect classifier at Stage 1.
func ComputeIncidentScore(in IncidentScoreInput) IncidentScore {
	now := in.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	rawStatus := strings.ToLower(strings.TrimSpace(in.RawStatus))
	displayStatus := rawStatus
	if strings.TrimSpace(in.DisplayStatus) != "" {
		displayStatus = strings.ToLower(strings.TrimSpace(in.DisplayStatus))
	}
	errKind := strings.ToLower(strings.TrimSpace(in.ErrorKind))

	out := IncidentScore{Value: 0, Reasons: nil}

	// Hard suppressions (operational states): do not treat as incidents.
	switch displayStatus {
	case "paused":
		out.Reasons = append(out.Reasons, IncidentScoreReasonStatusPaused)
		return out
	case "maintenance":
		out.Reasons = append(out.Reasons, IncidentScoreReasonStatusMaintenance)
		return out
	}

	// Base by status.
	switch rawStatus {
	case "down":
		out.Value = 0.70
		out.Reasons = append(out.Reasons, IncidentScoreReasonStatusDown)
	case "dns":
		out.Value = 0.55
		out.Reasons = append(out.Reasons, IncidentScoreReasonStatusDNS)
	case "up":
		out.Value = 0.0
	default:
		out.Value = 0.30
		out.Reasons = append(out.Reasons, IncidentScoreReasonStatusUnknown)
	}

	// Error kind adjustments.
	if errKind != "" && errKind != string(ErrorKindOK) {
		switch ErrorKind(errKind) {
		case ErrorKindTimeout:
			out.Value += 0.10
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorTimeout)
		case ErrorKindDNS:
			out.Value += 0.08
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorDNS)
		case ErrorKindConnect:
			out.Value += 0.08
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorConnect)
		case ErrorKindConnectionRefused:
			out.Value += 0.10
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorConnectionRefused)
		case ErrorKindNetworkUnreachable:
			out.Value += 0.10
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorNetworkUnreachable)
		case ErrorKindTLS:
			out.Value += 0.05
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorTLS)
		case ErrorKindHTTPStatus:
			out.Value += 0.05
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorHTTPStatus)
		case ErrorKindRequestFailed:
			out.Value += 0.08
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorRequestFailed)
		case ErrorKindInvalidURL:
			// Likely configuration issue rather than "incident".
			out.Value -= 0.10
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorInvalidURL)
		case ErrorKindPrivateBlocked:
			out.Value -= 0.15
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorPrivateBlocked)
		case ErrorKindRestrictedTarget:
			out.Value -= 0.15
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorRestrictedTarget)
		default:
			out.Value += 0.03
			out.Reasons = append(out.Reasons, IncidentScoreReasonErrorUnknown)
		}
	}

	// HTTP status code.
	// If the check is effectively UP (i.e. allowed status / success), do not treat 4xx/5xx as incident signals.
	// This avoids false "risk" for monitors intentionally configured to expect e.g. 404 as a healthy response.
	if in.StatusCode != nil && rawStatus != "up" {
		code := *in.StatusCode
		switch {
		case code >= 500 && code <= 599:
			out.Value += 0.15
			out.Reasons = append(out.Reasons, IncidentScoreReasonHTTP5xx)
		case code >= 400 && code <= 499:
			out.Value += 0.05
			out.Reasons = append(out.Reasons, IncidentScoreReasonHTTP4xx)
		}
	}

	// Latency hints (degradation signals).
	if in.LatencyMs >= 5000 {
		out.Value += 0.10
		out.Reasons = append(out.Reasons, IncidentScoreReasonLatencyVeryHigh)
	} else if in.LatencyMs >= 2000 {
		out.Value += 0.05
		out.Reasons = append(out.Reasons, IncidentScoreReasonLatencyHigh)
	}

	// Temporal hints from previous state.
	if in.Prev != nil {
		prevRaw := strings.ToLower(strings.TrimSpace(in.Prev.LastResultStatus))
		interval := time.Duration(max(1, in.Monitor.IntervalSec)) * time.Second

		// "Flapping": when we see DOWN shortly after an UP, reduce confidence.
		if rawStatus == "down" && in.Prev.LastUpAt != nil && now.Sub(in.Prev.LastUpAt.UTC()) <= 2*interval {
			out.Value -= 0.15
			out.Reasons = append(out.Reasons, IncidentScoreReasonFlapping)
		}

		// Down duration increases confidence.
		if rawStatus == "down" {
			if in.Prev.LastDownAt != nil {
				dur := now.Sub(in.Prev.LastDownAt.UTC())
				switch {
				case dur >= 30*time.Minute:
					out.Value += 0.20
					out.Reasons = append(out.Reasons, IncidentScoreReasonDownDurationSevere)
				case dur >= 5*time.Minute:
					out.Value += 0.10
					out.Reasons = append(out.Reasons, IncidentScoreReasonDownDurationLong)
				}
			} else if prevRaw == "down" && in.Prev.LastCheckedAt != nil {
				// Best-effort fallback: assume it has been down at least since the last check.
				dur := now.Sub(in.Prev.LastCheckedAt.UTC())
				if dur >= 5*time.Minute {
					out.Value += 0.08
					out.Reasons = append(out.Reasons, IncidentScoreReasonDownDurationLong)
				}
			}
		}

		// After recovery, drop quickly: keep a tiny residual score only for a very short window.
		if rawStatus == "up" && prevRaw == "down" && in.Prev.LastDownAt != nil {
			if now.Sub(in.Prev.LastDownAt.UTC()) <= 2*interval {
				out.Value = math.Max(out.Value, 0.05)
				out.Reasons = append(out.Reasons, IncidentScoreReasonRecentRecovery)
			}
		}
	}

	out.Value = clamp01(out.Value)
	out.Reasons = uniqNonEmpty(out.Reasons)
	return out
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

func uniqNonEmpty(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func max(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
