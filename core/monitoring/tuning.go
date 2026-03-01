package monitoring

import "time"

type Tuning struct {
	JitterPercent        int
	JitterMaxSeconds     int
	StatsLogInterval     time.Duration
}

func normalizeTuning(t Tuning) Tuning {
	if t.JitterPercent < 0 {
		t.JitterPercent = 0
	}
	if t.JitterPercent > 50 {
		t.JitterPercent = 50
	}
	if t.JitterMaxSeconds < 0 {
		t.JitterMaxSeconds = 0
	}
	if t.StatsLogInterval < 0 {
		t.StatsLogInterval = 0
	}
	return t
}

