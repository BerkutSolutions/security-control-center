package tests

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestSystemRoleProtected(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "roles.db")}
	logger := utils.NewLogger()
	db, _ := store.NewDB(cfg, logger)
	defer db.Close()
	_ = store.ApplyMigrations(context.Background(), db, logger)
	rs := store.NewRolesStore(db)
	role := &store.Role{Name: "custom", Permissions: []string{"docs.view"}, BuiltIn: false}
	id, err := rs.Create(context.Background(), role)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := rs.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete custom: %v", err)
	}
	builtin := &store.Role{Name: "builtin_x", Permissions: []string{"docs.view"}, BuiltIn: true}
	if _, err := rs.Create(context.Background(), builtin); err != nil {
		t.Fatalf("create builtin: %v", err)
	}
	if err := rs.Delete(context.Background(), builtin.ID); err == nil {
		t.Fatalf("expected delete built-in to fail")
	}
}

func TestCreateFromTemplatePermissions(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "roles.db")}
	logger := utils.NewLogger()
	db, _ := store.NewDB(cfg, logger)
	defer db.Close()
	_ = store.ApplyMigrations(context.Background(), db, logger)
	rs := store.NewRolesStore(db)
	role := &store.Role{Name: "doc_editor_clone", Permissions: []string{"docs.view", "docs.edit"}, BuiltIn: false}
	if _, err := rs.Create(context.Background(), role); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, _ := rs.FindByName(context.Background(), "doc_editor_clone")
	if len(got.Permissions) != 2 {
		t.Fatalf("permissions not saved")
	}
}

func TestCreateRoleFromTemplateAPI(t *testing.T) {
	h, roles, cleanup := setupRolesHandler(t)
	defer cleanup()
	req := httptest.NewRequest("POST", "/api/accounts/roles/from-template", bytes.NewBufferString(`{"template_id":"doc_viewer","name":"custom_viewer","description":"docs only"}`))
	rr := httptest.NewRecorder()
	h.CreateRoleFromTemplate(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	created, _ := roles.FindByName(context.Background(), "custom_viewer")
	if created == nil {
		t.Fatalf("role not created")
	}
	if !created.Template {
		t.Fatalf("expected template flag set")
	}
	for _, p := range []string{"docs.view", "docs.versions.view"} {
		if !containsPermission(created.Permissions, p) {
			t.Fatalf("missing permission %s", p)
		}
	}
}

func TestCreateRoleFromTemplateInvalid(t *testing.T) {
	h, roles, cleanup := setupRolesHandler(t)
	defer cleanup()
	req := httptest.NewRequest("POST", "/api/accounts/roles/from-template", bytes.NewBufferString(`{"template_id":"unknown","name":"bad"}`))
	rr := httptest.NewRecorder()
	h.CreateRoleFromTemplate(rr, req)
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 404/400, got %d", rr.Code)
	}
	if role, _ := roles.FindByName(context.Background(), "bad"); role != nil {
		t.Fatalf("unexpected role created for invalid template")
	}
}

func setupRolesHandler(t *testing.T) (*handlers.AccountsHandler, store.RolesStore, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "roles.db")}
	logger := utils.NewLogger()
	db, _ := store.NewDB(cfg, logger)
	t.Cleanup(func() { _ = db.Close() })
	_ = store.ApplyMigrations(context.Background(), db, logger)
	roles := store.NewRolesStore(db)
	audits := store.NewAuditStore(db)
	h := handlers.NewAccountsHandler(nil, nil, roles, nil, nil, nil, cfg, audits, logger, nil)
	return h, roles, func() { _ = db.Close() }
}

func containsPermission(perms []string, target string) bool {
	for _, p := range perms {
		if strings.EqualFold(p, target) {
			return true
		}
	}
	return false
}
