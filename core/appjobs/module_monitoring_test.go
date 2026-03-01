package appjobs

import (
	"context"
	"testing"
	"time"

	"berkut-scc/core/store"
)

func queryCount(t *testing.T, db queryRower, table string) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(1) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestMonitoringFullResetDeletesOnlyMonitoringData(t *testing.T) {
	db := mustTestDB(t)
	monStore := store.NewMonitoringStore(db)

	ctx := context.Background()
	monitorID, err := monStore.CreateMonitor(ctx, &store.Monitor{
		Name:             "m1",
		Type:             "http",
		URL:              "http://example.com",
		Method:           "GET",
		IntervalSec:      60,
		TimeoutSec:       1,
		Retries:          1,
		RetryIntervalSec: 1,
		AllowedStatus:    []string{"200-299"},
		IsActive:         true,
		IsPaused:         false,
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	retryAt := now.Add(5 * time.Minute)
	if err := monStore.UpsertMonitorState(ctx, &store.MonitorState{
		MonitorID:        monitorID,
		Status:           "down",
		LastResultStatus: "down",
		LastCheckedAt:    &now,
		RetryAt:          &retryAt,
		RetryAttempt:     1,
		LastAttemptAt:    &now,
		LastErrorKind:    "timeout",
		LastError:        "monitoring.error.timeout",
	}); err != nil {
		t.Fatalf("upsert state: %v", err)
	}
	if _, err := monStore.AddMetric(ctx, &store.MonitorMetric{MonitorID: monitorID, TS: now, LatencyMs: 123, OK: false}); err != nil {
		t.Fatalf("add metric: %v", err)
	}
	if _, err := monStore.AddEvent(ctx, &store.MonitorEvent{MonitorID: monitorID, TS: now, EventType: "down", Message: "m"}); err != nil {
		t.Fatalf("add event: %v", err)
	}

	// Insert unrelated business data (docs) to ensure monitoring reset doesn't wipe other modules.
	_, err = db.ExecContext(ctx, `
		INSERT INTO docs(title, status, classification_level, classification_tags, reg_number, doc_type, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		"Doc 1", "draft", 0, "[]", "R-1", "document", now, now,
	)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	if queryCount(t, db, "monitors") != 1 {
		t.Fatalf("expected 1 monitor before reset")
	}
	if queryCount(t, db, "monitor_state") != 1 {
		t.Fatalf("expected 1 state before reset")
	}
	if queryCount(t, db, "monitor_metrics") != 1 {
		t.Fatalf("expected 1 metric before reset")
	}
	if queryCount(t, db, "monitor_events") != 1 {
		t.Fatalf("expected 1 event before reset")
	}
	if queryCount(t, db, "docs") != 1 {
		t.Fatalf("expected 1 doc before reset")
	}

	reg := DefaultModuleRegistry()
	mod := reg.Get("monitoring")
	if mod == nil {
		t.Fatalf("monitoring module missing")
	}
	deps := ModuleDeps{DB: db, Modules: store.NewAppModuleStateStore(db), NowUTC: now}
	if _, err := mod.FullReset(ctx, deps); err != nil {
		t.Fatalf("full reset: %v", err)
	}

	if queryCount(t, db, "monitors") != 0 {
		t.Fatalf("expected monitors deleted")
	}
	if queryCount(t, db, "monitor_state") != 0 {
		t.Fatalf("expected monitor_state deleted")
	}
	if queryCount(t, db, "monitor_metrics") != 0 {
		t.Fatalf("expected monitor_metrics deleted")
	}
	if queryCount(t, db, "monitor_events") != 0 {
		t.Fatalf("expected monitor_events deleted")
	}
	if queryCount(t, db, "docs") != 1 {
		t.Fatalf("expected docs preserved")
	}
}

func TestMonitoringPartialAdaptDoesNotDeleteMonitorAndResetsRetryState(t *testing.T) {
	db := mustTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)

	// Insert a monitor with messy fields so partial adapt has work to do.
	res, err := db.ExecContext(ctx, `
		INSERT INTO monitors(name, type, url, method, request_body_type, headers_json, interval_sec, timeout_sec, retries, retry_interval_sec, allowed_status_json, is_active, is_paused, tags_json, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"m1", "   ", " http://example.com/test ", "GET", "none", "{}", 60, 1, 1, 1, `["200-299"]`, 1, 0, "[]", now, now,
	)
	if err != nil {
		t.Fatalf("insert monitor: %v", err)
	}
	monitorID, _ := res.LastInsertId()
	retryAt := now.Add(10 * time.Minute)
	lastAttempt := now.Add(-1 * time.Minute)
	if err := store.NewMonitoringStore(db).UpsertMonitorState(ctx, &store.MonitorState{
		MonitorID:     monitorID,
		Status:        "down",
		RetryAt:       &retryAt,
		RetryAttempt:  2,
		LastAttemptAt: &lastAttempt,
		LastErrorKind: " timeout ",
	}); err != nil {
		t.Fatalf("upsert state: %v", err)
	}

	reg := DefaultModuleRegistry()
	mod := reg.Get("monitoring")
	if mod == nil {
		t.Fatalf("monitoring module missing")
	}
	deps := ModuleDeps{DB: db, Modules: store.NewAppModuleStateStore(db), NowUTC: now}
	if _, err := mod.PartialAdapt(ctx, deps); err != nil {
		t.Fatalf("partial adapt: %v", err)
	}

	// Monitor must remain.
	if queryCount(t, db, "monitors") != 1 {
		t.Fatalf("expected monitor preserved")
	}

	// URL trimmed and type defaulted.
	var typ, url string
	if err := db.QueryRowContext(ctx, `SELECT type, url FROM monitors WHERE id=?`, monitorID).Scan(&typ, &url); err != nil {
		t.Fatalf("read monitor: %v", err)
	}
	if typ != "http" {
		t.Fatalf("expected type defaulted to http, got %q", typ)
	}
	if url != "http://example.com/test" {
		t.Fatalf("expected url trimmed, got %q", url)
	}

	// Retry fields reset.
	var retryAttempt int
	var retryAtDB any
	var lastErrKind string
	if err := db.QueryRowContext(ctx, `SELECT retry_attempt, retry_at, last_error_kind FROM monitor_state WHERE monitor_id=?`, monitorID).Scan(&retryAttempt, &retryAtDB, &lastErrKind); err != nil {
		t.Fatalf("read state: %v", err)
	}
	if retryAttempt != 0 {
		t.Fatalf("expected retry_attempt reset to 0, got %d", retryAttempt)
	}
	if retryAtDB != nil {
		t.Fatalf("expected retry_at cleared, got %v", retryAtDB)
	}
	if lastErrKind != "" {
		t.Fatalf("expected last_error_kind cleared, got %q", lastErrKind)
	}
}

