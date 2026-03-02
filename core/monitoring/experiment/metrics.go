package experiment

import "time"

type FalseOpenCriterion struct {
	// For replay mode where ground truth is unknown, an incident opened for an outage shorter than this window
	// is counted as a "likely false open".
	ShortOutageWindowSec int `json:"short_outage_window_sec"`
}

type Metrics struct {
	Observations int `json:"observations"`

	Outages int `json:"outages"`

	Opens  int `json:"opens"`
	Closes int `json:"closes"`

	Actions int `json:"actions"`

	OpenedOutages int `json:"opened_outages"`
	FalseOpens    int `json:"false_opens"`

	AvgTimeToOpenSec float64 `json:"avg_time_to_open_sec"`
	P50TimeToOpenSec float64 `json:"p50_time_to_open_sec"`
	P95TimeToOpenSec float64 `json:"p95_time_to_open_sec"`
}

type OutageResult struct {
	Index int `json:"index"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	OpenedAt *time.Time `json:"opened_at,omitempty"`
	ClosedAt *time.Time `json:"closed_at,omitempty"`

	TimeToOpenSec *int `json:"time_to_open_sec,omitempty"`

	DurationSec int `json:"duration_sec"`

	FalseOpen bool `json:"false_open"`

	// TruthOutage is set only in simulate mode, where scenario generator provides ground truth.
	TruthOutage *bool `json:"truth_outage,omitempty"`
	// LikelyRealOutage is a replay-mode heuristic (based on outage duration) used for fitting and loss calculations.
	LikelyRealOutage bool `json:"likely_real_outage,omitempty"`
}

func finalizeTimeToOpenStats(m *Metrics, outages []OutageResult) {
	var vals []float64
	for _, o := range outages {
		if o.TimeToOpenSec == nil {
			continue
		}
		sec := float64(*o.TimeToOpenSec)
		if sec < 0 {
			continue
		}
		vals = append(vals, sec)
	}
	m.AvgTimeToOpenSec = mean(vals)
	m.P50TimeToOpenSec = quantile(vals, 0.50)
	m.P95TimeToOpenSec = quantile(vals, 0.95)
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func quantile(vals []float64, q float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	if q <= 0 {
		return min(vals)
	}
	if q >= 1 {
		return max(vals)
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	quickSort(cp, 0, len(cp)-1)
	pos := q * float64(len(cp)-1)
	i := int(pos)
	if i >= len(cp)-1 {
		return cp[len(cp)-1]
	}
	frac := pos - float64(i)
	return cp[i]*(1-frac) + cp[i+1]*frac
}

func min(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals {
		if v < m {
			m = v
		}
	}
	return m
}

func max(vals []float64) float64 {
	m := vals[0]
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func quickSort(a []float64, lo, hi int) {
	if lo >= hi {
		return
	}
	p := partition(a, lo, hi)
	quickSort(a, lo, p-1)
	quickSort(a, p+1, hi)
}

func partition(a []float64, lo, hi int) int {
	pivot := a[hi]
	i := lo
	for j := lo; j < hi; j++ {
		if a[j] <= pivot {
			a[i], a[j] = a[j], a[i]
			i++
		}
	}
	a[i], a[hi] = a[hi], a[i]
	return i
}
