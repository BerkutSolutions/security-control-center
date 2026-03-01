package monitoring

import (
	"testing"
	"time"
)

func TestJitterDelaySecondsDeterministicAndBounded(t *testing.T) {
	tuning := Tuning{JitterPercent: 20, JitterMaxSeconds: 10}
	interval := 30 // window=6
	d1 := jitterDelaySeconds(123, interval, tuning.JitterPercent, tuning.JitterMaxSeconds)
	d2 := jitterDelaySeconds(123, interval, tuning.JitterPercent, tuning.JitterMaxSeconds)
	if d1 != d2 {
		t.Fatalf("expected deterministic delay, got %d and %d", d1, d2)
	}
	if d1 < 0 || d1 > 6 {
		t.Fatalf("expected delay within [0..6], got %d", d1)
	}
}

func TestJitterDelaySecondsCappedByMaxSeconds(t *testing.T) {
	tuning := Tuning{JitterPercent: 20, JitterMaxSeconds: 3}
	interval := 300 // window=60, cap=3
	d := jitterDelaySeconds(999, interval, tuning.JitterPercent, tuning.JitterMaxSeconds)
	if d < 0 || d > 3 {
		t.Fatalf("expected delay within [0..3], got %d", d)
	}
}

func TestEligibleAtUsesCreatedAtWhenNeverChecked(t *testing.T) {
	tuning := Tuning{JitterPercent: 20, JitterMaxSeconds: 10}
	created := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	at := eligibleAt(1, created, nil, 30, tuning)
	if at.Before(created) {
		t.Fatalf("expected eligibleAt >= createdAt, got %s < %s", at, created)
	}
	if at.After(created.Add(7 * time.Second)) {
		t.Fatalf("expected eligibleAt within jitter window for interval 30s, got %s", at)
	}
}

func TestEligibleAtUsesLastCheckedAtPlusInterval(t *testing.T) {
	tuning := Tuning{JitterPercent: 20, JitterMaxSeconds: 10}
	created := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	last := time.Date(2026, 2, 28, 12, 0, 10, 0, time.UTC)
	at := eligibleAt(42, created, &last, 30, tuning)
	min := last.Add(30 * time.Second)
	max := min.Add(7 * time.Second)
	if at.Before(min) || at.After(max) {
		t.Fatalf("expected eligibleAt within [%s..%s], got %s", min, max, at)
	}
}

