package monitoring

import (
	"context"
	"database/sql"
	"path/filepath"
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
