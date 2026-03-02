package monitoring

import (
	"testing"
	"time"

	"berkut-scc/core/store"
)

func TestComputeIncidentScore_MonotonicDownDuration(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	mon := store.Monitor{IntervalSec: 60}

	shortPrev := &store.MonitorState{
		LastResultStatus: "down",
		LastDownAt:       ptrTime(now.Add(-2 * time.Minute)),
		LastCheckedAt:    ptrTime(now.Add(-1 * time.Minute)),
	}
	longPrev := &store.MonitorState{
		LastResultStatus: "down",
		LastDownAt:       ptrTime(now.Add(-40 * time.Minute)),
		LastCheckedAt:    ptrTime(now.Add(-1 * time.Minute)),
	}

	short := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "down",
		ErrorKind: string(ErrorKindTimeout),
		Now:       now,
		Prev:      shortPrev,
		Monitor:   mon,
	})
	long := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "down",
		ErrorKind: string(ErrorKindTimeout),
		Now:       now,
		Prev:      longPrev,
		Monitor:   mon,
	})
	if long.Value <= short.Value {
		t.Fatalf("expected long down duration to score higher: long=%v short=%v", long.Value, short.Value)
	}
	if long.Value < 0 || long.Value > 1 || short.Value < 0 || short.Value > 1 {
		t.Fatalf("scores must be clamped to [0..1]: long=%v short=%v", long.Value, short.Value)
	}
}

func TestComputeIncidentScore_DropsAfterRecovery(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	mon := store.Monitor{IntervalSec: 60}

	prev := &store.MonitorState{
		LastResultStatus: "down",
		LastDownAt:       ptrTime(now.Add(-10 * time.Minute)),
		LastCheckedAt:    ptrTime(now.Add(-1 * time.Minute)),
	}

	down := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "down",
		ErrorKind: string(ErrorKindConnect),
		Now:       now,
		Prev:      prev,
		Monitor:   mon,
	})
	up := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "up",
		ErrorKind: string(ErrorKindOK),
		Now:       now,
		Prev:      prev,
		Monitor:   mon,
	})

	if !(down.Value > up.Value) {
		t.Fatalf("expected score to drop after recovery: down=%v up=%v", down.Value, up.Value)
	}
	if up.Value > 0.20 {
		t.Fatalf("expected recovery score to be low: %v", up.Value)
	}
}

func TestComputeIncidentScore_FlappingPenalty(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	mon := store.Monitor{IntervalSec: 60}

	flapPrev := &store.MonitorState{
		LastResultStatus: "up",
		LastUpAt:         ptrTime(now.Add(-30 * time.Second)),
		LastCheckedAt:    ptrTime(now.Add(-30 * time.Second)),
	}
	stablePrev := &store.MonitorState{
		LastResultStatus: "down",
		LastDownAt:       ptrTime(now.Add(-20 * time.Minute)),
		LastCheckedAt:    ptrTime(now.Add(-1 * time.Minute)),
	}

	flap := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "down",
		ErrorKind: string(ErrorKindRequestFailed),
		Now:       now,
		Prev:      flapPrev,
		Monitor:   mon,
	})
	stable := ComputeIncidentScore(IncidentScoreInput{
		RawStatus: "down",
		ErrorKind: string(ErrorKindRequestFailed),
		Now:       now,
		Prev:      stablePrev,
		Monitor:   mon,
	})

	if flap.Value >= stable.Value {
		t.Fatalf("expected flapping to reduce confidence: flap=%v stable=%v", flap.Value, stable.Value)
	}
}

func TestComputeIncidentScore_Deterministic(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	code := 503
	in := IncidentScoreInput{
		RawStatus:  "down",
		ErrorKind:  string(ErrorKindHTTPStatus),
		StatusCode: &code,
		LatencyMs:  2500,
		Now:        now,
		Prev: &store.MonitorState{
			LastResultStatus: "down",
			LastDownAt:       ptrTime(now.Add(-6 * time.Minute)),
			LastCheckedAt:    ptrTime(now.Add(-1 * time.Minute)),
		},
		Monitor: store.Monitor{IntervalSec: 60},
	}
	a := ComputeIncidentScore(in)
	b := ComputeIncidentScore(in)
	if a.Value != b.Value {
		t.Fatalf("expected deterministic score: a=%v b=%v", a.Value, b.Value)
	}
	if len(a.Reasons) != len(b.Reasons) {
		t.Fatalf("expected deterministic reasons length: a=%v b=%v", a.Reasons, b.Reasons)
	}
}

func TestComputeIncidentScore_HTTP404AllowedDoesNotIncreaseRiskWhenUp(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)
	code := 404
	res := ComputeIncidentScore(IncidentScoreInput{
		RawStatus:  "up",
		ErrorKind:  string(ErrorKindOK),
		StatusCode: &code,
		LatencyMs:  0,
		Now:        now,
		Prev:       nil,
		Monitor:    store.Monitor{IntervalSec: 60},
	})
	if res.Value != 0 {
		t.Fatalf("expected score 0 for UP with allowed 404, got=%v reasons=%v", res.Value, res.Reasons)
	}
	for _, r := range res.Reasons {
		if r == IncidentScoreReasonHTTP4xx {
			t.Fatalf("expected no http_4xx reason for UP, got reasons=%v", res.Reasons)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
