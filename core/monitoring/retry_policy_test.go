package monitoring

import (
	"context"
	"testing"
	"time"

	"berkut-scc/core/store"
)

func TestDecideRetrySchedulesOnRetryableError(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	m := store.Monitor{
		ID:               42,
		Retries:          2,
		RetryIntervalSec: 5,
		RetryAttempt:     0,
	}
	res := CheckResult{CheckedAt: now, OK: false}
	dec := DecideRetry(now, m, store.MonitorSettings{DefaultRetryIntervalSec: 5}, res, context.DeadlineExceeded)
	if !dec.Scheduled {
		t.Fatalf("expected retry scheduled")
	}
	if dec.RetryAttempt != 1 {
		t.Fatalf("expected retry_attempt=1, got %d", dec.RetryAttempt)
	}
	wantDelay := retryDelay(m.ID, 1, 5*time.Second)
	wantAt := now.Add(wantDelay).UTC()
	if dec.RetryAt == nil || !dec.RetryAt.Equal(wantAt) {
		t.Fatalf("expected retry_at=%s, got %v", wantAt, dec.RetryAt)
	}
}

func TestDecideRetryStopsAfterRetriesExhausted(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	m := store.Monitor{
		ID:               42,
		Retries:          2,
		RetryIntervalSec: 5,
		RetryAttempt:     2,
	}
	res := CheckResult{CheckedAt: now, OK: false}
	dec := DecideRetry(now, m, store.MonitorSettings{DefaultRetryIntervalSec: 5}, res, context.DeadlineExceeded)
	if dec.Scheduled {
		t.Fatalf("expected no retry scheduled")
	}
	if dec.RetryAt != nil || dec.RetryAttempt != 0 {
		t.Fatalf("expected retry state cleared, got retry_at=%v retry_attempt=%d", dec.RetryAt, dec.RetryAttempt)
	}
}

func TestDecideRetryDoesNotScheduleOnHTTPStatusFailure(t *testing.T) {
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)
	m := store.Monitor{
		ID:               1,
		Retries:          3,
		RetryIntervalSec: 5,
		RetryAttempt:     0,
	}
	res := CheckResult{CheckedAt: now, OK: false, Error: "status_500"}
	dec := DecideRetry(now, m, store.MonitorSettings{DefaultRetryIntervalSec: 5}, res, nil)
	if dec.Scheduled {
		t.Fatalf("expected no retry scheduled")
	}
}

func TestRetryStartBudget(t *testing.T) {
	if got := retryStartBudget(20, 1); got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
	if got := retryStartBudget(1, 10); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := retryStartBudget(20, 0); got != 20 {
		t.Fatalf("expected 20, got %d", got)
	}
}
