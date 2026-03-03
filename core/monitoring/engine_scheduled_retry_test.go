package monitoring

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func mustMonitoringTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "tmp.db"), Pepper: "pepper", DBURL: ""}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEngineDoesNotSleepInsideRetrySlot(t *testing.T) {
	db := mustMonitoringTestDB(t)
	monStore := store.NewMonitoringStore(db)
	engine := NewEngine(monStore, nil)

	settings := store.MonitorSettings{
		EngineEnabled:           true,
		MaxConcurrentChecks:     1,
		AllowPrivateNetworks:    true,
		DefaultRetryIntervalSec: 2,
	}
	engine.ensureSemaphore(settings.MaxConcurrentChecks)

	now := time.Now().UTC()
	create := func(name string) int64 {
		id, err := monStore.CreateMonitor(context.Background(), &store.Monitor{
			Name:             name,
			Type:             "http",
			URL:              "http://example.invalid",
			Method:           "GET",
			IntervalSec:      30,
			TimeoutSec:       1,
			Retries:          2,
			RetryIntervalSec: 2,
			AllowedStatus:    []string{"200-299"},
			IsActive:         true,
			IsPaused:         false,
			CreatedBy:        1,
			CreatedAt:        now.Add(-time.Hour),
			UpdatedAt:        now.Add(-time.Hour),
		})
		if err != nil {
			t.Fatalf("create monitor: %v", err)
		}
		lastChecked := now.Add(-time.Hour)
		_ = monStore.UpsertMonitorState(context.Background(), &store.MonitorState{
			MonitorID:        id,
			Status:           "down",
			LastResultStatus: "down",
			LastCheckedAt:    &lastChecked,
		})
		return id
	}

	_ = create("m1")
	_ = create("m2")

	var calls atomic.Int64
	callCh := make(chan int64, 10)
	var failedID atomic.Int64
	engine.attemptFn = func(ctx context.Context, m store.Monitor, settings store.MonitorSettings) (CheckResult, error) {
		n := calls.Add(1)
		callCh <- m.ID
		if n == 1 {
			failedID.Store(m.ID)
			return CheckResult{CheckedAt: time.Now().UTC(), OK: false}, context.DeadlineExceeded
		}
		return CheckResult{CheckedAt: time.Now().UTC(), OK: true}, nil
	}

	engine.runDueChecksAt(context.Background(), settings, now)

	select {
	case <-callCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected first attempt to start")
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		engine.runDueChecksAt(context.Background(), settings, time.Now().UTC())
		select {
		case <-callCh:
			goto gotSecond
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatalf("expected second attempt to start quickly (slot should be freed without retry sleep)")

gotSecond:
	id := failedID.Load()
	if id <= 0 {
		t.Fatalf("missing failed monitor id")
	}
	state, err := monStore.GetMonitorState(context.Background(), id)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if state == nil || state.RetryAt == nil || state.RetryAttempt != 1 {
		t.Fatalf("expected retry scheduled in state (retry_at set, retry_attempt=1), got %+v", state)
	}
	if !state.RetryAt.After(now) {
		t.Fatalf("expected retry_at in the future, got %s", state.RetryAt)
	}
}

func TestRetryableFailureDoesNotFlipStatusUntilExhausted(t *testing.T) {
	db := mustMonitoringTestDB(t)
	monStore := store.NewMonitoringStore(db)
	engine := NewEngine(monStore, nil)

	settings := store.MonitorSettings{
		EngineEnabled:           true,
		MaxConcurrentChecks:     1,
		AllowPrivateNetworks:    true,
		DefaultRetryIntervalSec: 1,
		IssueEscalateMinutes:    10,
		NotifyUpConfirmations:   2,
	}

	base := time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)
	id, err := monStore.CreateMonitor(context.Background(), &store.Monitor{
		Name:             "b2c",
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
	lastUp := base.Add(-5 * time.Minute)
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
	_, _, _ = engine.runCheck(context.Background(), *mon, settings)

	st1, err := monStore.GetMonitorState(context.Background(), id)
	if err != nil || st1 == nil {
		t.Fatalf("state after first failure: %v", err)
	}
	if st1.RetryAt == nil || st1.RetryAttempt != 1 {
		t.Fatalf("expected retry scheduled after first failure, got %+v", st1)
	}
	if got := strings.ToLower(strings.TrimSpace(st1.LastResultStatus)); got != "up" {
		t.Fatalf("expected status to remain up while retry is scheduled, got %q", got)
	}

	// Second attempt: retry budget exhausted, failure becomes confirmed.
	now2 := st1.RetryAt.UTC().Add(2 * time.Second)
	due, err := monStore.ListDueMonitors(context.Background(), now2)
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

	st2, err := monStore.GetMonitorState(context.Background(), id)
	if err != nil || st2 == nil {
		t.Fatalf("state after confirmed failure: %v", err)
	}
	if st2.RetryAt != nil || st2.RetryAttempt != 0 {
		t.Fatalf("expected retry cleared after exhaustion, got %+v", st2)
	}
	if got := strings.ToLower(strings.TrimSpace(st2.LastResultStatus)); got != "issue" {
		t.Fatalf("expected confirmed issue after retries exhausted, got %q", got)
	}
}

func TestLongTimeoutEscalatesIssueToDown(t *testing.T) {
	db := mustMonitoringTestDB(t)
	monStore := store.NewMonitoringStore(db)
	engine := NewEngine(monStore, nil)

	settings := store.MonitorSettings{
		EngineEnabled:           true,
		MaxConcurrentChecks:     1,
		AllowPrivateNetworks:    true,
		DefaultRetryIntervalSec: 1,
	}

	base := time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)
	id, err := monStore.CreateMonitor(context.Background(), &store.Monitor{
		Name:             "timeout-long",
		Type:             "http",
		URL:              "http://example.invalid",
		Method:           "GET",
		IntervalSec:      60,
		TimeoutSec:       1,
		Retries:          2,
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

	// Simulate a monitor that hasn't been UP for >10 minutes but still sits in ISSUE state.
	lastUp := base.Add(-12 * time.Minute)
	lastChecked := base.Add(-time.Minute)
	if err := monStore.UpsertMonitorState(context.Background(), &store.MonitorState{
		MonitorID:        id,
		Status:           "issue",
		LastResultStatus: "issue",
		LastCheckedAt:    &lastChecked,
		LastUpAt:         &lastUp,
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	engine.attemptFn = func(ctx context.Context, m store.Monitor, settings store.MonitorSettings) (CheckResult, error) {
		return CheckResult{CheckedAt: base, OK: false}, context.DeadlineExceeded
	}

	mon, err := monStore.GetMonitor(context.Background(), id)
	if err != nil || mon == nil {
		t.Fatalf("get monitor: %v", err)
	}
	_, _, _ = engine.runCheck(context.Background(), *mon, settings)

	st, err := monStore.GetMonitorState(context.Background(), id)
	if err != nil || st == nil {
		t.Fatalf("get state: %v", err)
	}
	if got := strings.ToLower(strings.TrimSpace(st.LastResultStatus)); got != "down" {
		t.Fatalf("expected escalation to down, got %q", got)
	}
	if st.RetryAt != nil || st.RetryAttempt != 0 {
		t.Fatalf("expected retries cleared on escalation, got retry_at=%v retry_attempt=%d", st.RetryAt, st.RetryAttempt)
	}
}
