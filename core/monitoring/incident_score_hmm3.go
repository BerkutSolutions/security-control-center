package monitoring

import (
	"strings"

	"berkut-scc/core/monitoring/incidenthmm"
)

type IncidentHMM3Result struct {
	Score       IncidentScore
	Posterior   []float64
	State       string
	Observation string
}

const (
	IncidentScoringModelHeuristic = "heuristic"
	IncidentScoringModelHMM3      = "hmm3"
)

func ComputeIncidentScoreHMM3(in IncidentScoreInput) IncidentHMM3Result {
	return ComputeIncidentScoreHMM3WithParams(in, incidenthmm.DefaultParams())
}

func ComputeIncidentScoreHMM3WithParams(in IncidentScoreInput, p incidenthmm.Params) IncidentHMM3Result {
	display := strings.ToLower(strings.TrimSpace(in.DisplayStatus))
	if display == "" {
		display = strings.ToLower(strings.TrimSpace(in.RawStatus))
	}
	switch display {
	case "paused":
		return IncidentHMM3Result{
			Score: IncidentScore{Value: 0, Reasons: []string{IncidentScoreReasonStatusPaused}},
		}
	case "maintenance":
		return IncidentHMM3Result{
			Score: IncidentScore{Value: 0, Reasons: []string{IncidentScoreReasonStatusMaintenance}},
		}
	}

	prior := p.Prior
	if in.Prev != nil && len(in.Prev.IncidentScorePosterior) == 3 {
		prior = incidenthmm.Vec3{
			in.Prev.IncidentScorePosterior[0],
			in.Prev.IncidentScorePosterior[1],
			in.Prev.IncidentScorePosterior[2],
		}
	}

	obs := classifyHMMObservation(in)
	post := incidenthmm.FilterStep(prior, obs, p)
	state := incidenthmm.ArgMax(post)

	outage := post[incidenthmm.StateOutage]
	degraded := post[incidenthmm.StateDegraded]
	scoreVal := clamp01(outage + 0.5*degraded)

	reasons := []string{
		IncidentScoreReasonModelHMM3,
		IncidentScoreReasonHMMObsPrefix + obs.String(),
		IncidentScoreReasonHMMStatePrefix + state.String(),
	}

	return IncidentHMM3Result{
		Score: IncidentScore{
			Value:   scoreVal,
			Reasons: uniqNonEmpty(reasons),
		},
		Posterior:   []float64{post[0], post[1], post[2]},
		State:       state.String(),
		Observation: obs.String(),
	}
}

func classifyHMMObservation(in IncidentScoreInput) incidenthmm.Observation {
	raw := strings.ToLower(strings.TrimSpace(in.RawStatus))
	kind := strings.ToLower(strings.TrimSpace(in.ErrorKind))

	// "UP" can still carry degradation signals (latency / 5xx).
	if raw == "up" {
		if in.StatusCode != nil && *in.StatusCode >= 500 && *in.StatusCode <= 599 {
			return incidenthmm.ObsHTTP5xx
		}
		if in.LatencyMs >= 5000 {
			return incidenthmm.ObsLatencyVeryHigh
		}
		if in.LatencyMs >= 2000 {
			return incidenthmm.ObsLatencyHigh
		}
		return incidenthmm.ObsOK
	}

	// DNS status is a strong signal.
	if raw == "dns" || kind == string(ErrorKindDNS) {
		return incidenthmm.ObsDNS
	}

	switch ErrorKind(kind) {
	case ErrorKindTimeout:
		return incidenthmm.ObsTimeout
	case ErrorKindConnect:
		return incidenthmm.ObsConnect
	case ErrorKindConnectionRefused:
		return incidenthmm.ObsConnectionRefused
	}

	if in.StatusCode != nil && *in.StatusCode >= 500 && *in.StatusCode <= 599 {
		return incidenthmm.ObsHTTP5xx
	}

	return incidenthmm.ObsOtherDown
}
