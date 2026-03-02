package incidenthmm

import "testing"

func TestFilterStep_SumsToOne(t *testing.T) {
	p := DefaultParams()
	prior := p.Prior
	next := FilterStep(prior, ObsOK, p)
	sum := next[0] + next[1] + next[2]
	if sum < 0.999999 || sum > 1.000001 {
		t.Fatalf("sum=%v, next=%v", sum, next)
	}
}

func TestFilterStep_OutageProbabilityIncreasesOnRepeatedTimeouts(t *testing.T) {
	p := DefaultParams()
	post := p.Prior
	prevOut := post[StateOutage]
	for i := 0; i < 5; i++ {
		post = FilterStep(post, ObsTimeout, p)
		if post[StateOutage] < prevOut-1e-9 {
			t.Fatalf("outage prob decreased at step %d: prev=%v next=%v", i, prevOut, post[StateOutage])
		}
		prevOut = post[StateOutage]
	}
	if post[StateOutage] < 0.5 {
		t.Fatalf("expected outage prob to become high, got=%v", post[StateOutage])
	}
}

func TestFilterStep_OutageProbabilityDropsAfterOK(t *testing.T) {
	p := DefaultParams()
	post := p.Prior
	for i := 0; i < 4; i++ {
		post = FilterStep(post, ObsTimeout, p)
	}
	before := post[StateOutage]
	post = FilterStep(post, ObsOK, p)
	after := post[StateOutage]
	if after >= before {
		t.Fatalf("expected outage prob to drop after OK: before=%v after=%v", before, after)
	}
}

func TestFilterStep_DegradedRespondsToLatency(t *testing.T) {
	p := DefaultParams()
	prior := p.Prior
	post := FilterStep(prior, ObsLatencyHigh, p)
	if post[StateDegraded] <= 0.10 {
		t.Fatalf("expected degraded prob to become non-trivial after latency_high: post=%v", post)
	}
	if post[StateDegraded]+post[StateOutage] <= prior[StateDegraded]+prior[StateOutage] {
		t.Fatalf("expected non-normal mass to increase after latency_high: prior=%v post=%v", prior, post)
	}
	post2 := FilterStep(post, ObsLatencyVeryHigh, p)
	if post2[StateDegraded]+post2[StateOutage] < post[StateDegraded]+post[StateOutage]-1e-9 {
		t.Fatalf("expected non-normal mass not to decrease after latency_very_high: before=%v after=%v", post, post2)
	}
}
