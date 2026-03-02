package experiment

import (
	"math/rand"
	"time"
)

type ScenarioMode string

const (
	ScenarioMixed   ScenarioMode = "mixed"
	ScenarioOutage  ScenarioMode = "outage"
	ScenarioFlap    ScenarioMode = "flap"
	ScenarioDegrade ScenarioMode = "degrade"
)

type ScenarioConfig struct {
	Mode     ScenarioMode  `json:"mode"`
	Seed     int64         `json:"seed"`
	StartAt  time.Time     `json:"start_at"`
	Duration time.Duration `json:"duration"`
	Step     time.Duration `json:"step"`
	Latency  int           `json:"latency_ms"`
}

type LabeledObservation struct {
	Observation
	TruthOutage bool `json:"truth_outage"`
}

type Observation struct {
	TS         time.Time `json:"ts"`
	LatencyMs  int       `json:"latency_ms"`
	OK         bool      `json:"ok"`
	StatusCode *int      `json:"status_code,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func GenerateScenario(cfg ScenarioConfig) []LabeledObservation {
	start := cfg.StartAt
	if start.IsZero() {
		start = time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	}
	step := cfg.Step
	if step <= 0 {
		step = 30 * time.Second
	}
	dur := cfg.Duration
	if dur <= 0 {
		dur = 60 * time.Minute
	}
	baseLatency := cfg.Latency
	if baseLatency <= 0 {
		baseLatency = 120
	}

	rng := rand.New(rand.NewSource(cfg.Seed))
	var out []LabeledObservation
	n := int(dur / step)
	if n <= 0 {
		n = 1
	}

	for i := 0; i <= n; i++ {
		ts := start.Add(time.Duration(i) * step)
		item := LabeledObservation{
			Observation: Observation{
				TS:        ts,
				LatencyMs: jitterLatency(rng, baseLatency, 0.15),
				OK:        true,
			},
			TruthOutage: false,
		}

		switch cfg.Mode {
		case ScenarioOutage:
			// One outage in the middle.
			if i >= n/3 && i <= (2*n)/3 {
				item = makeDown(ts, rng)
			}
		case ScenarioFlap:
			// Alternating down/up bursts.
			if i >= n/4 && i <= (3*n)/4 {
				if (i/2)%2 == 0 {
					item = makeDown(ts, rng)
				}
			}
		case ScenarioDegrade:
			// Degradation without hard outage (high latency, occasional 5xx but mostly OK).
			if i >= n/4 && i <= (3*n)/4 {
				item.Observation.LatencyMs = jitterLatency(rng, 2500, 0.25)
				if rng.Float64() < 0.08 {
					code := 503
					item.Observation.OK = false
					item.Observation.StatusCode = &code
					item.Observation.Error = "status_503"
					item.TruthOutage = false
				}
			}
		case ScenarioMixed, "":
			// Normal -> flap -> outage -> normal.
			if i >= n/5 && i < (2*n)/5 {
				if i%3 == 0 {
					item = makeDown(ts, rng)
				}
			}
			if i >= (3*n)/5 && i <= (4*n)/5 {
				item = makeDown(ts, rng)
			}
		default:
			// Unknown -> treat as mixed.
			if i >= (3*n)/5 && i <= (4*n)/5 {
				item = makeDown(ts, rng)
			}
		}

		out = append(out, item)
	}
	return out
}

func makeDown(ts time.Time, rng *rand.Rand) LabeledObservation {
	kind := rng.Float64()
	lat := jitterLatency(rng, 0, 0)
	var code *int
	err := "monitoring.error.requestFailed"
	if kind < 0.50 {
		c := 503
		code = &c
		err = "status_503"
	} else if kind < 0.70 {
		err = "monitoring.error.timeout"
	} else if kind < 0.85 {
		err = "no such host"
	} else {
		err = "connect: connection refused"
	}
	return LabeledObservation{
		Observation: Observation{
			TS:         ts,
			LatencyMs:  lat,
			OK:         false,
			StatusCode: code,
			Error:      err,
		},
		TruthOutage: true,
	}
}

func jitterLatency(rng *rand.Rand, base int, frac float64) int {
	if base <= 0 {
		return 0
	}
	if frac <= 0 {
		return base
	}
	delta := int(float64(base) * frac)
	if delta <= 0 {
		return base
	}
	return base - delta + rng.Intn(2*delta+1)
}
