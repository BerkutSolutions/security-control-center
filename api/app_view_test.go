package api

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestAppViewLogsControlsTabToAudit(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "app_view.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	audits := store.NewAuditStore(db)
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "r1", Permissions: []rbac.Permission{"controls.view"}},
	})
	s := &Server{policy: policy, audits: audits}

	body := []byte(`{"page":"registry","tab":"frameworks","path":"/registry/frameworks"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/app/view", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"r1"},
	}))
	rr := httptest.NewRecorder()
	s.appView(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rr.Code, rr.Body.String())
	}

	items, err := audits.List(context.Background())
	if err != nil {
		t.Fatalf("audit list: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected at least one audit record")
	}
	if items[0].Action != "registry.tab.frameworks.view" {
		t.Fatalf("expected action registry.tab.frameworks.view, got %q", items[0].Action)
	}
	if items[0].Username != "u1" {
		t.Fatalf("expected username u1, got %q", items[0].Username)
	}
	if items[0].Details != "" {
		t.Fatalf("expected empty details, got %q", items[0].Details)
	}
}

func TestAppViewEnforcesPermissions(t *testing.T) {
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "r0", Permissions: nil},
	})
	s := &Server{policy: policy, audits: store.NewAuditStore(mustTestDB(t))}

	body := []byte(`{"page":"registry","tab":"overview","path":"/registry/overview"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/app/view", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   2,
		Username: "u2",
		Roles:    []string{"r0"},
	}))
	rr := httptest.NewRecorder()
	s.appView(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func mustTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "tmp.db"), Pepper: "pepper"}
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
