package monitoring

import (
	"context"
	"strings"
	"testing"
	"time"

	"berkut-scc/core/store"
)

func TestContinuousOutageDoesNotSpamEventsOnRetryExhaustion(t *testing.T) {
	db := mustMonitoringTestDB(t)
	monStore := store.NewMonitoringStore(db)
	engine := NewEngine(monStore, nil)

	settings := store.MonitorSettings{
		EngineEnabled:           true,
		AllowPrivateNetworks:    true,
		DefaultRetryIntervalSec: 1,
		IssueEscalateMinutes:    10,
	}

	base := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	id, err := monStore.CreateMonitor(context.Background(), &store.Monitor{
		Name:             "registry",
		Type:             "http",
		URL:              "http://example.invalid",
		Method:           "GET",
		IntervalSec:      60,
		TimeoutSec:       1,
		Retries:          1,
		RetryIntervalSec: 1,
		AllowedStatus:    []string{"200-299"},
		IsActive:         true,
		IsPaused:         false,
		CreatedBy:        1,
		CreatedAt:        base.Add(-time.Hour),
		UpdatedAt:        base.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	// Last UP is long ago, so a sustained timeout should escalate to DOWN on confirmation.
	lastUp := base.Add(-12 * time.Minute)
	lastChecked := base.Add(-5 * time.Minute)
	if err := monStore.UpsertMonitorState(context.Background(), &store.MonitorState{
		MonitorID:        id,
		Status:           "up",
		LastResultStatus: "up",
		LastCheckedAt:    &lastChecked,
		LastUpAt:         &lastUp,
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	attempts := 0
	engine.attemptFn = func(ctx context.Context, m store.Monitor, settings store.MonitorSettings) (CheckResult, error) {
		attempts++
		return CheckResult{CheckedAt: base.Add(time.Duration(attempts) * time.Second), OK: false}, context.DeadlineExceeded
	}

	mon, err := monStore.GetMonitor(context.Background(), id)
	if err != nil || mon == nil {
		t.Fatalf("get monitor: %v", err)
	}

	// 1) First failure schedules retry (no event yet).
	_, _, _ = engine.runCheck(context.Background(), *mon, settings)
	// 2) Second failure exhausts retry budget => confirmed and escalated to DOWN => one DOWN event.
	// Make the monitor due for retry.
	st, _ := monStore.GetMonitorState(context.Background(), id)
	if st == nil || st.RetryAt == nil {
		t.Fatalf("expected retry scheduled after first failure, got %+v", st)
	}
	due, err := monStore.ListDueMonitors(context.Background(), st.RetryAt.UTC().Add(2*time.Second))
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	var retryMon *store.Monitor
	for i := range due {
		if due[i].ID == id {
			retryMon = &due[i]
			break
		}
	}
	if retryMon == nil {
		t.Fatalf("expected monitor to be due for retry")
	}
	_, _, _ = engine.runCheck(context.Background(), *retryMon, settings)

	events, err := monStore.ListEvents(context.Background(), id, base.Add(-time.Hour))
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	downCount := 0
	for _, ev := range events {
		if strings.ToLower(strings.TrimSpace(ev.EventType)) == "down" {
			downCount++
		}
	}
	if downCount != 1 {
		t.Fatalf("expected exactly one DOWN event for continuous outage, got %d events=%+v", downCount, events)
	}

	// 3) Start a new scheduled check window while still DOWN: should not create another DOWN event.
	nextWindow := base.Add(2 * time.Minute)
	_ = monStore.UpsertMonitorState(context.Background(), &store.MonitorState{
		MonitorID:        id,
		Status:           "down",
		LastResultStatus: "down",
		LastCheckedAt:    &nextWindow,
		LastUpAt:         &lastUp,
	})
	_, _, _ = engine.runCheck(context.Background(), *mon, settings)
	st2, _ := monStore.GetMonitorState(context.Background(), id)
	if st2 == nil || st2.RetryAt == nil {
		t.Fatalf("expected retry scheduled in ongoing outage, got %+v", st2)
	}
	due2, _ := monStore.ListDueMonitors(context.Background(), st2.RetryAt.UTC().Add(2*time.Second))
	var retryMon2 *store.Monitor
	for i := range due2 {
		if due2[i].ID == id {
			retryMon2 = &due2[i]
			break
		}
	}
	if retryMon2 == nil {
		t.Fatalf("expected monitor to be due for retry (window 2)")
	}
	_, _, _ = engine.runCheck(context.Background(), *retryMon2, settings)

	events2, _ := monStore.ListEvents(context.Background(), id, base.Add(-time.Hour))
	downCount2 := 0
	for _, ev := range events2 {
		if strings.ToLower(strings.TrimSpace(ev.EventType)) == "down" {
			downCount2++
		}
	}
	if downCount2 != 1 {
		t.Fatalf("expected still one DOWN event after retry exhaustion in ongoing outage, got %d", downCount2)
	}
}

