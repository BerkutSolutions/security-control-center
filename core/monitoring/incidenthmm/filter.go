package incidenthmm

import "math"

// FilterStep performs one online Bayesian update for a 3-state Hidden Markov Model.
//
// prior is the posterior from the previous step (or Params.Prior if this is the first observation).
// obs is the classified observation at current time step.
func FilterStep(prior Vec3, obs Observation, p Params) Vec3 {
	prior = NormalizeVec3(prior)
	if obs < 0 || obs >= obsCount {
		obs = ObsOtherDown
	}

	var predicted Vec3
	for to := 0; to < int(stateCount); to++ {
		sum := 0.0
		for from := 0; from < int(stateCount); from++ {
			sum += prior[from] * p.A[from][to]
		}
		predicted[to] = sum
	}

	var unnorm Vec3
	for s := 0; s < int(stateCount); s++ {
		unnorm[s] = predicted[s] * p.B[s][obs]
	}

	next := NormalizeVec3(unnorm)
	// If emissions were degenerate, fallback to predicted distribution.
	if next[0] == 0 && next[1] == 0 && next[2] == 0 {
		next = NormalizeVec3(predicted)
	}
	return next
}

func NormalizeVec3(v Vec3) Vec3 {
	sum := 0.0
	for i := 0; i < int(stateCount); i++ {
		if v[i] > 0 {
			sum += v[i]
		}
	}
	if sum <= 0 || math.IsNaN(sum) || math.IsInf(sum, 0) {
		return Vec3{1, 0, 0}
	}
	for i := 0; i < int(stateCount); i++ {
		if v[i] < 0 || math.IsNaN(v[i]) || math.IsInf(v[i], 0) {
			v[i] = 0
		}
		v[i] /= sum
	}
	return v
}

func ArgMax(v Vec3) State {
	best := StateNormal
	bestVal := v[0]
	for i := 1; i < int(stateCount); i++ {
		if v[i] > bestVal {
			bestVal = v[i]
			best = State(i)
		}
	}
	return best
}

