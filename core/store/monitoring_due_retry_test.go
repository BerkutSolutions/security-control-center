package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/utils"
)

func mustTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "tmp.db"), Pepper: "pepper", DBURL: ""}
	logger := utils.NewLogger()
	db, err := NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestListDueMonitorsRespectsRetryAt(t *testing.T) {
	db := mustTestDB(t)
	s := NewMonitoringStore(db)
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	id, err := s.CreateMonitor(context.Background(), &Monitor{
		Name:             "m1",
		Type:             "http",
		URL:              "http://example.com",
		Method:           "GET",
		IntervalSec:      60,
		TimeoutSec:       1,
		Retries:          2,
		RetryIntervalSec: 5,
		AllowedStatus:    []string{"200-299"},
		IsActive:         true,
		IsPaused:         false,
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	lastChecked := now.Add(-2 * time.Hour)
	retryAt := now.Add(5 * time.Minute)
	if err := s.UpsertMonitorState(context.Background(), &MonitorState{
		MonitorID:        id,
		Status:           "down",
		LastResultStatus: "down",
		LastCheckedAt:    &lastChecked,
		RetryAt:          &retryAt,
		RetryAttempt:     1,
		LastAttemptAt:    &lastChecked,
		LastErrorKind:    "timeout",
		LastError:        "monitoring.error.timeout",
	}); err != nil {
		t.Fatalf("upsert state: %v", err)
	}

	list1, err := s.ListDueMonitors(context.Background(), now)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}
	if len(list1) != 0 {
		t.Fatalf("expected no due monitors before retry_at, got %d", len(list1))
	}

	list2, err := s.ListDueMonitors(context.Background(), now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("list due 2: %v", err)
	}
	if len(list2) != 1 || list2[0].ID != id {
		t.Fatalf("expected due monitor by retry_at")
	}
}
