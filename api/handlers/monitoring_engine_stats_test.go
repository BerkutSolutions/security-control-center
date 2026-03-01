package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/monitoring"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func mustTestDB(t *testing.T) *sql.DB {
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

func TestMonitoringEngineStatsEnforcesPermission(t *testing.T) {
	db := mustTestDB(t)
	monStore := store.NewMonitoringStore(db)
	engine := monitoring.NewEngineWithDeps(monStore, nil, nil, "", nil, nil, utils.NewLogger())
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "r0", Permissions: nil},
		{Name: "r1", Permissions: []rbac.Permission{"monitoring.view"}},
	})
	h := NewMonitoringHandler(monStore, store.NewUsersStore(db), store.NewAuditStore(db), engine, policy, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/engine/stats", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"r0"},
	}))
	rr := httptest.NewRecorder()
	h.GetEngineStats(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/monitoring/engine/stats", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   2,
		Username: "u2",
		Roles:    []string{"r1"},
	}))
	rr2 := httptest.NewRecorder()
	h.GetEngineStats(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr2.Code, rr2.Body.String())
	}
}

