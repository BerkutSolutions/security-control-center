package experiment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
)

type Mode string

const (
	ModeReplay   Mode = "replay"
	ModeSimulate Mode = "simulate"
)

type PolicyKind string

const (
	PolicyLegacyDown PolicyKind = "legacy_down"
	PolicyScoreV1    PolicyKind = "score_v1"
	PolicyScoreHMM3  PolicyKind = "score_hmm3"
)

type PolicyConfig struct {
	Kind           PolicyKind `json:"kind"`
	CloseOnUp      bool       `json:"close_on_up"`
	OpenThreshold  float64    `json:"open_threshold,omitempty"`
	CloseThreshold float64    `json:"close_threshold,omitempty"`
	Confirmations  int        `json:"confirmations,omitempty"`
	HMM3DiagBoost  float64    `json:"hmm3_diag_boost,omitempty"`
}

type PolicyResult struct {
	Name      string             `json:"name"`
	Config    PolicyConfig       `json:"config"`
	Criterion FalseOpenCriterion `json:"false_open_criterion"`
	Metrics   Metrics            `json:"metrics"`
	Outages   []OutageResult     `json:"outages"`
}

type Result struct {
	GeneratedAt time.Time `json:"generated_at"`
	Mode        Mode      `json:"mode"`
	MonitorID   int64     `json:"monitor_id"`
	Since       time.Time `json:"since"`

	Scenario *ScenarioConfig `json:"scenario,omitempty"`

	Policies []PolicyResult `json:"policies"`
}

type RunConfig struct {
	Mode              Mode
	Monitor           store.Monitor
	Settings          store.MonitorSettings
	Since             time.Time
	Replay            []Observation
	Simulate          []LabeledObservation
	Scenario          *ScenarioConfig
	ShortOutageWindow time.Duration
}

func Run(ctx context.Context, cfg RunConfig) (Result, error) {
	if cfg.Monitor.ID <= 0 {
		return Result{}, errors.New("monitor_id is required")
	}
	if cfg.ShortOutageWindow <= 0 {
		cfg.ShortOutageWindow = 120 * time.Second
	}
	if cfg.Since.IsZero() {
		cfg.Since = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}

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
		return Result{}, fmt.Errorf("unknown mode: %s", cfg.Mode)
	}

	legacy := runPolicy(ctx, "baseline", PolicyConfig{
		Kind:      PolicyLegacyDown,
		CloseOnUp: cfg.Settings.AutoIncidentCloseOnUp,
	}, cfg.Monitor, cfg.Settings, obs, truth, cfg.ShortOutageWindow, cfg.Mode)

	score := runPolicy(ctx, "scoring", PolicyConfig{
		Kind:           PolicyScoreV1,
		CloseOnUp:      cfg.Settings.AutoIncidentCloseOnUp,
		OpenThreshold:  cfg.Settings.IncidentScoreOpenThreshold,
		CloseThreshold: cfg.Settings.IncidentScoreCloseThreshold,
		Confirmations:  cfg.Settings.IncidentScoreOpenConfirmations,
	}, cfg.Monitor, cfg.Settings, obs, truth, cfg.ShortOutageWindow, cfg.Mode)

	hmm := runPolicy(ctx, "hmm3", PolicyConfig{
		Kind:           PolicyScoreHMM3,
		CloseOnUp:      cfg.Settings.AutoIncidentCloseOnUp,
		OpenThreshold:  cfg.Settings.IncidentScoreOpenThreshold,
		CloseThreshold: cfg.Settings.IncidentScoreCloseThreshold,
		Confirmations:  cfg.Settings.IncidentScoreOpenConfirmations,
	}, cfg.Monitor, cfg.Settings, obs, truth, cfg.ShortOutageWindow, cfg.Mode)

	return Result{
		GeneratedAt: time.Now().UTC(),
		Mode:        cfg.Mode,
		MonitorID:   cfg.Monitor.ID,
		Since:       cfg.Since,
		Scenario:    cfg.Scenario,
		Policies:    []PolicyResult{legacy, score, hmm},
	}, nil
}

func runPolicy(ctx context.Context, name string, pc PolicyConfig, mon store.Monitor, settings store.MonitorSettings, obs []Observation, truth []bool, shortOutage time.Duration, mode Mode) PolicyResult {
	_ = ctx

	criterion := FalseOpenCriterion{ShortOutageWindowSec: int(shortOutage.Seconds())}
	var outages []OutageResult
	var metrics Metrics
	metrics.Observations = len(obs)

	openThreshold := clamp01(pc.OpenThreshold)
	closeThreshold := clamp01(pc.CloseThreshold)
	confirmations := pc.Confirmations
	if confirmations <= 0 {
		confirmations = 1
	}

	inOutage := false
	outageIdx := -1
	outageStart := time.Time{}
	outageTruth := false
	var openedAt *time.Time
	var closedAt *time.Time
	opened := false
	seq := 0

	var prev *store.MonitorState
	prevRaw := "up"

	closeOutage := func(end time.Time) {
		if !inOutage {
			return
		}
		outage := OutageResult{
			Index:       outageIdx,
			StartedAt:   outageStart,
			EndedAt:     end,
			OpenedAt:    openedAt,
			ClosedAt:    closedAt,
			DurationSec: int(end.Sub(outageStart).Seconds()),
		}
		if mode == ModeSimulate {
			val := outageTruth
			outage.TruthOutage = &val
			outage.LikelyRealOutage = outageTruth
		} else {
			outage.TruthOutage = nil
			outage.LikelyRealOutage = shortOutage > 0 && end.Sub(outageStart) >= shortOutage
		}
		if outage.OpenedAt != nil {
			metrics.OpenedOutages++
			sec := int(outage.OpenedAt.Sub(outageStart).Seconds())
			if sec < 0 {
				sec = 0
			}
			outage.TimeToOpenSec = &sec
		}
		if outage.OpenedAt != nil {
			if outageTruth {
				// Simulated mode: truth-based; nothing to do.
			} else {
				// Replay mode: use short-outage heuristic.
				if shortOutage > 0 && end.Sub(outageStart) < shortOutage {
					outage.FalseOpen = true
					metrics.FalseOpens++
					outage.LikelyRealOutage = false
				}
			}
		}
		outages = append(outages, outage)
		inOutage = false
		outageStart = time.Time{}
		outageTruth = false
		openedAt = nil
		closedAt = nil
	}

	for i, o := range obs {
		raw, kind := classifyRawStatus(o)
		now := o.TS
		if now.IsZero() {
			now = time.Now().UTC()
		}
		isBad := raw != "up"
		if isBad {
			seq++
		} else {
			seq = 0
		}

		if isBad && !inOutage {
			inOutage = true
			outageIdx++
			outageStart = now
			outageTruth = i < len(truth) && truth[i]
			metrics.Outages++
		}
		if !isBad && inOutage {
			// Close outage window before processing recovery close.
			closeOutage(now)
		}

		score := 0.0
		var reasons []string
		var hmmRes monitoring.IncidentHMM3Result
		if pc.Kind == PolicyScoreV1 {
			inp := monitoring.IncidentScoreInput{
				RawStatus:  raw,
				ErrorKind:  string(kind),
				StatusCode: o.StatusCode,
				LatencyMs:  o.LatencyMs,
				Now:        now,
				Prev:       prev,
				Monitor:    mon,
				Settings:   settings,
			}
			res := monitoring.ComputeIncidentScore(inp)
			score = res.Value
			reasons = res.Reasons
		} else if pc.Kind == PolicyScoreHMM3 {
			inp := monitoring.IncidentScoreInput{
				RawStatus:  raw,
				ErrorKind:  string(kind),
				StatusCode: o.StatusCode,
				LatencyMs:  o.LatencyMs,
				Now:        now,
				Prev:       prev,
				Monitor:    mon,
				Settings:   settings,
			}
			params := monitoring.HMM3DefaultParamsWithDiagBoost(pc.HMM3DiagBoost)
			hmmRes = monitoring.ComputeIncidentScoreHMM3WithParams(inp, params)
			score = hmmRes.Score.Value
			reasons = hmmRes.Score.Reasons
		}

		next := simulateState(prev, now, raw, kind, o)
		if pc.Kind == PolicyScoreV1 {
			next.IncidentScore = &score
			next.IncidentScoreReasons = reasons
		} else if pc.Kind == PolicyScoreHMM3 {
			next.IncidentScore = &score
			next.IncidentScoreReasons = reasons
			next.IncidentScorePosterior = hmmRes.Posterior
			next.IncidentScoreState = hmmRes.State
			next.IncidentScoreObs = hmmRes.Observation
		}
		prev = next

		switch pc.Kind {
		case PolicyLegacyDown:
			if raw == "down" && prevRaw != "down" && !opened {
				opened = true
				ts := now
				openedAt = &ts
				metrics.Opens++
				metrics.Actions++
			}
			if raw == "up" && prevRaw == "down" && opened && pc.CloseOnUp {
				opened = false
				ts := now
				closedAt = &ts
				metrics.Closes++
				metrics.Actions++
			}
		case PolicyScoreV1:
			if !opened && raw != "up" && score >= openThreshold && seq >= confirmations {
				opened = true
				ts := now
				openedAt = &ts
				metrics.Opens++
				metrics.Actions++
				_ = reasons
			}
			if opened && (score <= closeThreshold || (raw == "up" && pc.CloseOnUp)) {
				opened = false
				ts := now
				closedAt = &ts
				metrics.Closes++
				metrics.Actions++
			}
		case PolicyScoreHMM3:
			if !opened && raw != "up" && score >= openThreshold && seq >= confirmations {
				opened = true
				ts := now
				openedAt = &ts
				metrics.Opens++
				metrics.Actions++
				_ = reasons
			}
			if opened && (score <= closeThreshold || (raw == "up" && pc.CloseOnUp)) {
				opened = false
				ts := now
				closedAt = &ts
				metrics.Closes++
				metrics.Actions++
			}
		}
		prevRaw = raw
	}

	// Finalize last outage if it never recovered.
	if inOutage && len(obs) > 0 {
		closeOutage(obs[len(obs)-1].TS)
	}

	metrics.Actions = metrics.Opens + metrics.Closes
	for _, o := range outages {
		if o.FalseOpen {
			// already counted
			continue
		}
	}
	finalizeTimeToOpenStats(&metrics, outages)

	return PolicyResult{
		Name:      name,
		Config:    pc,
		Criterion: criterion,
		Metrics:   metrics,
		Outages:   outages,
	}
}
