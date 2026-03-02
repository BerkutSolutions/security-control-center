package experiment

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type LossWeights struct {
	FalseOpen float64 `json:"w_false_open"`
	Miss      float64 `json:"w_miss"`
	DelaySec  float64 `json:"w_delay_sec"`
	Noise     float64 `json:"w_noise"`
}

func DefaultLossWeights() LossWeights {
	return LossWeights{
		FalseOpen: 10,
		Miss:      50,
		DelaySec:  0.05,
		Noise:     1,
	}
}

type LossBreakdown struct {
	FalseOpens  int     `json:"false_opens"`
	Misses      int     `json:"misses"`
	DelaySecSum float64 `json:"delay_sec_sum"`
	DelaySecAvg float64 `json:"delay_sec_avg"`
	Actions     int     `json:"actions"`
	Value       float64 `json:"loss"`
}

type FitGrid struct {
	OpenThresholds  []float64 `json:"open_thresholds"`
	CloseThresholds []float64 `json:"close_thresholds"`
	Confirmations   []int     `json:"confirmations"`
	HMM3DiagBoosts  []float64 `json:"hmm3_diag_boosts,omitempty"`
}

type FitCandidate struct {
	Policy PolicyKind   `json:"policy"`
	Config PolicyConfig `json:"config"`
	Loss   LossBreakdown `json:"loss"`
}

type FitResult struct {
	GeneratedAt time.Time `json:"generated_at"`
	Mode        Mode      `json:"mode"`
	MonitorID   int64     `json:"monitor_id"`
	Since       time.Time `json:"since"`

	Scenario *ScenarioConfig `json:"scenario,omitempty"`

	Weights LossWeights `json:"weights"`
	Grid    FitGrid     `json:"grid"`

	Best []FitCandidate `json:"best"`
	Top  []FitCandidate `json:"top"`
}

type FitConfig struct {
	Weights LossWeights
	Grid    FitGrid
	TopN    int
}

func Fit(ctx context.Context, runCfg RunConfig, fc FitConfig) (FitResult, error) {
	_ = ctx
	if runCfg.Monitor.ID <= 0 {
		return FitResult{}, errors.New("monitor_id is required")
	}
	if runCfg.Since.IsZero() {
		runCfg.Since = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if runCfg.ShortOutageWindow <= 0 {
		runCfg.ShortOutageWindow = 120 * time.Second
	}
	if fc.TopN <= 0 {
		fc.TopN = 10
	}
	if fc.Weights == (LossWeights{}) {
		fc.Weights = DefaultLossWeights()
	}
	if len(fc.Grid.OpenThresholds) == 0 || len(fc.Grid.CloseThresholds) == 0 || len(fc.Grid.Confirmations) == 0 {
		return FitResult{}, errors.New("fit grid is empty")
	}
	if len(fc.Grid.HMM3DiagBoosts) == 0 {
		fc.Grid.HMM3DiagBoosts = []float64{0}
	}

	obs, truth, err := obsAndTruth(runCfg)
	if err != nil {
		return FitResult{}, err
	}

	settings := runCfg.Settings
	monitor := runCfg.Monitor

	policies := []PolicyKind{PolicyScoreV1, PolicyScoreHMM3}
	bestByPolicy := map[PolicyKind]FitCandidate{}
	var top []FitCandidate

	for _, pk := range policies {
		bestByPolicy[pk] = FitCandidate{
			Policy: pk,
			Config: PolicyConfig{Kind: pk},
			Loss:   LossBreakdown{Value: 1e18},
		}
	}

	for _, open := range fc.Grid.OpenThresholds {
		open = clamp01(open)
		for _, close := range fc.Grid.CloseThresholds {
			close = clamp01(close)
			if close >= open {
				continue
			}
			for _, conf := range fc.Grid.Confirmations {
				if conf <= 0 {
					continue
				}
				for _, pk := range policies {
					boosts := []float64{0}
					if pk == PolicyScoreHMM3 {
						boosts = fc.Grid.HMM3DiagBoosts
					}
					for _, boost := range boosts {
						pc := PolicyConfig{
							Kind:           pk,
							CloseOnUp:      settings.AutoIncidentCloseOnUp,
							OpenThreshold:  open,
							CloseThreshold: close,
							Confirmations:  conf,
							HMM3DiagBoost:  boost,
						}
						pr := runPolicy(ctx, fmt.Sprintf("%s_fit", pk), pc, monitor, settings, obs, truth, runCfg.ShortOutageWindow, runCfg.Mode)
						lb := ComputeLoss(pr, fc.Weights)
						cand := FitCandidate{Policy: pk, Config: pc, Loss: lb}

						best := bestByPolicy[pk]
						if cand.Loss.Value < best.Loss.Value {
							bestByPolicy[pk] = cand
						}
						top = insertTopN(top, cand, fc.TopN)
					}
				}
			}
		}
	}

	var best []FitCandidate
	for _, pk := range policies {
		best = append(best, bestByPolicy[pk])
	}

	return FitResult{
		GeneratedAt: time.Now().UTC(),
		Mode:        runCfg.Mode,
		MonitorID:   runCfg.Monitor.ID,
		Since:       runCfg.Since,
		Scenario:    runCfg.Scenario,
		Weights:     fc.Weights,
		Grid:        fc.Grid,
		Best:        best,
		Top:         top,
	}, nil
}

func obsAndTruth(cfg RunConfig) ([]Observation, []bool, error) {
	var obs []Observation
	var truth []bool
	switch cfg.Mode {
	case ModeReplay:
		obs = cfg.Replay
		truth = make([]bool, len(obs))
	case ModeSimulate:
		for _, item := range cfg.Simulate {
			obs = append(obs, item.Observation)
			truth = append(truth, item.TruthOutage)
		}
	default:
		return nil, nil, fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
	return obs, truth, nil
}

func ComputeLoss(pr PolicyResult, w LossWeights) LossBreakdown {
	lb := LossBreakdown{
		FalseOpens: pr.Metrics.FalseOpens,
		Actions:    pr.Metrics.Actions,
	}
	delaySum := 0.0
	delayCount := 0
	for _, o := range pr.Outages {
		real := o.LikelyRealOutage
		if o.TruthOutage != nil {
			real = *o.TruthOutage
		}
		if real && o.OpenedAt == nil {
			lb.Misses++
		}
		if real && o.TimeToOpenSec != nil && *o.TimeToOpenSec >= 0 {
			delaySum += float64(*o.TimeToOpenSec)
			delayCount++
		}
	}
	lb.DelaySecSum = delaySum
	if delayCount > 0 {
		lb.DelaySecAvg = delaySum / float64(delayCount)
	}
	lb.Value = w.FalseOpen*float64(lb.FalseOpens) +
		w.Miss*float64(lb.Misses) +
		w.DelaySec*lb.DelaySecSum +
		w.Noise*float64(lb.Actions)
	return lb
}

func insertTopN(list []FitCandidate, cand FitCandidate, n int) []FitCandidate {
	if n <= 0 {
		return list
	}
	// Keep list sorted by loss asc.
	pos := 0
	for pos < len(list) && list[pos].Loss.Value <= cand.Loss.Value {
		pos++
	}
	if pos >= n {
		return list
	}
	list = append(list, FitCandidate{})
	copy(list[pos+1:], list[pos:])
	list[pos] = cand
	if len(list) > n {
		list = list[:n]
	}
	return list
}
