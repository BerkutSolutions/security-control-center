package behavior

import "math"

type Metrics struct {
	SensitiveViews5m int
	Exports30m       int
	Denied10m        int
	Mutations5m      int
	Requests1m       int
	HistorySensitive int
	HistoryExports   int
	HistoryDenied    int
	HistoryMutations int
	HistoryRequests  int
	HistoryEvents    int
	IPNovelty        float64
	RecentlyVerified bool
}

type Result struct {
	Score    float64
	Trigger  bool
	Features map[string]float64
}

func Evaluate(m Metrics) Result {
	zSensitive := zScore(float64(m.SensitiveViews5m), histRate(float64(m.HistorySensitive), 7*24*12))
	zExports := zScore(float64(m.Exports30m), histRate(float64(m.HistoryExports), 7*24*2))
	zDenied := zScore(float64(m.Denied10m), histRate(float64(m.HistoryDenied), 7*24*6))
	zMutations := zScore(float64(m.Mutations5m), histRate(float64(m.HistoryMutations), 7*24*12))
	zReq := zScore(float64(m.Requests1m), histRate(float64(m.HistoryRequests), 7*24*60))

	burst := math.Max(0, (float64(m.SensitiveViews5m+m.Mutations5m)-12.0)/6.0)
	raw := -3.4 +
		0.85*zSensitive +
		1.2*zExports +
		1.4*zDenied +
		0.7*zMutations +
		0.55*zReq +
		0.9*clamp(m.IPNovelty, 0, 1) +
		0.6*burst

	score := sigmoid(raw)
	totalRecent := m.SensitiveViews5m + m.Exports30m + m.Denied10m + m.Mutations5m

	// False-positive suppressors.
	if totalRecent < 6 {
		score *= 0.55
	}
	if m.Denied10m == 0 && m.Exports30m == 0 && m.IPNovelty <= 0.01 {
		score *= 0.75
	}
	if m.RecentlyVerified {
		score *= 0.65
	}
	if m.HistoryEvents < 100 && m.Denied10m < 3 && m.Exports30m == 0 && (m.SensitiveViews5m+m.Mutations5m) < 18 && score > 0.9 {
		score = 0.9
	}
	score = clamp(score, 0, 1)

	riskSignal := (m.Exports30m + m.Denied10m) > 0
	highVolume := (m.SensitiveViews5m + m.Mutations5m) >= 14
	requestBurst := m.Requests1m >= 120
	trigger := score >= 0.88 && (riskSignal || highVolume || requestBurst)
	return Result{
		Score:   score,
		Trigger: trigger,
		Features: map[string]float64{
			"z_sensitive": zSensitive,
			"z_exports":   zExports,
			"z_denied":    zDenied,
			"z_mutations": zMutations,
			"z_requests":  zReq,
			"burst":       burst,
			"ip_novelty":  clamp(m.IPNovelty, 0, 1),
			"raw":         raw,
		},
	}
}

func histRate(total float64, windows int) float64 {
	if windows <= 0 {
		return 0
	}
	return total / float64(windows)
}

func zScore(x, mu float64) float64 {
	sigma := math.Sqrt(mu + 0.25)
	if sigma < 0.5 {
		sigma = 0.5
	}
	z := (x - mu) / sigma
	if z < 0 {
		return 0
	}
	return z
}

func sigmoid(x float64) float64 {
	if x >= 0 {
		z := math.Exp(-x)
		return 1 / (1 + z)
	}
	z := math.Exp(x)
	return z / (1 + z)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
