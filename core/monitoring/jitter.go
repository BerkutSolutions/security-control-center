package monitoring

import "time"

// eligibleAt returns the time when the monitor becomes eligible to run, applying deterministic jitter.
// The jitter is a per-monitor delay within a window derived from the interval. It is stable across restarts.
func eligibleAt(monitorID int64, createdAt time.Time, lastCheckedAt *time.Time, intervalSec int, t Tuning) time.Time {
	base := createdAt.UTC()
	if lastCheckedAt != nil && !lastCheckedAt.IsZero() && intervalSec > 0 {
		base = lastCheckedAt.UTC().Add(time.Duration(intervalSec) * time.Second)
	}
	delay := jitterDelaySeconds(monitorID, intervalSec, t.JitterPercent, t.JitterMaxSeconds)
	if delay <= 0 {
		return base
	}
	return base.Add(time.Duration(delay) * time.Second)
}

func jitterDelaySeconds(monitorID int64, intervalSec int, percent int, maxSeconds int) int {
	if percent <= 0 || intervalSec <= 0 {
		return 0
	}
	window := (intervalSec * percent) / 100
	if maxSeconds > 0 && window > maxSeconds {
		window = maxSeconds
	}
	if window <= 0 {
		return 0
	}
	v := splitmix64(uint64(monitorID))
	return int(v % uint64(window+1))
}

func splitmix64(x uint64) uint64 {
	// Deterministic 64-bit mixer (SplitMix64).
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

