package incidenthmm

import "math"

type Params struct {
	// A is a transition matrix: A[from][to].
	A [stateCount][stateCount]float64
	// B is an emission matrix: B[state][observation].
	B [stateCount][obsCount]float64
	// Prior is used when no previous posterior is available.
	Prior Vec3
}

func DefaultParams() Params {
	p := Params{
		A: [stateCount][stateCount]float64{
			// Normal -> {Normal, Degraded, Outage}
			{0.93, 0.06, 0.01},
			// Degraded -> {Normal, Degraded, Outage}
			{0.10, 0.80, 0.10},
			// Outage -> {Normal, Degraded, Outage}
			{0.05, 0.15, 0.80},
		},
		B: [stateCount][obsCount]float64{
			// Normal emits mostly OK, sometimes mild latency / rare failures.
			{
				0.950, // ok
				0.030, // latency_high
				0.005, // latency_very_high
				0.005, // http_5xx
				0.003, // timeout
				0.002, // dns
				0.002, // connect
				0.001, // connection_refused
				0.002, // other_down
			},
			// Degraded: elevated latency and some 5xx/timeouts, but still can be OK.
			{
				0.600, // ok
				0.200, // latency_high
				0.080, // latency_very_high
				0.050, // http_5xx
				0.030, // timeout
				0.010, // dns
				0.010, // connect
				0.010, // connection_refused
				0.010, // other_down
			},
			// Outage: failures dominate.
			{
				0.050, // ok
				0.010, // latency_high
				0.010, // latency_very_high
				0.100, // http_5xx
				0.350, // timeout
				0.150, // dns
				0.120, // connect
				0.120, // connection_refused
				0.090, // other_down
			},
		},
		Prior: Vec3{1, 0, 0},
	}
	p.Normalize()
	return p
}

// WithDiagBoost adjusts the transition matrix A by boosting (or damping) diagonal probabilities
// and renormalizing each row.
//
// boost=0 keeps A unchanged.
// boost>0 makes states more "sticky" (more likely to stay in the same state).
// boost in (-0.9..0) makes states less sticky.
func WithDiagBoost(base Params, boost float64) Params {
	// Keep it bounded to avoid extreme degeneracy.
	if boost > 10 {
		boost = 10
	}
	if boost < -0.9 {
		boost = -0.9
	}
	mult := 1.0 + boost
	for i := 0; i < int(stateCount); i++ {
		if mult <= 0 {
			// If boost makes mult non-positive (shouldn't due to clamp), fallback.
			continue
		}
		base.A[i][i] *= mult
	}
	base.Normalize()
	return base
}

func (p *Params) Normalize() {
	if p == nil {
		return
	}
	for i := 0; i < int(stateCount); i++ {
		normalizeRow(p.A[i][:])
		normalizeRow(p.B[i][:])
	}
	p.Prior = NormalizeVec3(p.Prior)
}

func (p Params) Validate() error {
	const eps = 1e-6
	for i := 0; i < int(stateCount); i++ {
		sumA := 0.0
		sumB := 0.0
		for j := 0; j < int(stateCount); j++ {
			if p.A[i][j] < 0 {
				return ErrInvalidParams
			}
			sumA += p.A[i][j]
		}
		for o := 0; o < int(obsCount); o++ {
			if p.B[i][o] < 0 {
				return ErrInvalidParams
			}
			sumB += p.B[i][o]
		}
		if math.Abs(sumA-1) > eps {
			return ErrInvalidParams
		}
		if math.Abs(sumB-1) > eps {
			return ErrInvalidParams
		}
	}
	sumP := p.Prior[0] + p.Prior[1] + p.Prior[2]
	if sumP <= 0 || math.Abs(sumP-1) > eps {
		return ErrInvalidParams
	}
	for i := 0; i < int(stateCount); i++ {
		if p.Prior[i] < 0 {
			return ErrInvalidParams
		}
	}
	return nil
}

var ErrInvalidParams = errorString("incidenthmm: invalid params")

type errorString string

func (e errorString) Error() string { return string(e) }

func normalizeRow(row []float64) {
	sum := 0.0
	for _, v := range row {
		if v > 0 {
			sum += v
		}
	}
	if sum <= 0 {
		n := float64(len(row))
		if n <= 0 {
			return
		}
		for i := range row {
			row[i] = 1 / n
		}
		return
	}
	for i := range row {
		if row[i] < 0 {
			row[i] = 0
		}
		row[i] /= sum
	}
}
