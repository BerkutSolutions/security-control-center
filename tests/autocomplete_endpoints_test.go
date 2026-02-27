package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestAssetsAutocompleteRejectsInvalidField(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "assets_auto.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	users := store.NewUsersStore(db)
	if _, err := users.Create(context.Background(), &store.User{Username: "u1", Active: true}, nil); err != nil {
		t.Fatalf("create user: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewAssetsHandler(store.NewAssetsStore(db), store.NewSoftwareStore(db), users, store.NewAuditStore(db), policy)

	req := httptest.NewRequest(http.MethodGet, "/api/assets/autocomplete?field=bad", nil)
	req = makeSessionContext(req, "u1", 1, []string{"superadmin"})
	rr := httptest.NewRecorder()
	h.Autocomplete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "assets.autocomplete.fieldInvalid") {
		t.Fatalf("expected assets.autocomplete.fieldInvalid, got %q", rr.Body.String())
	}
}

func TestFindingsAutocompleteRejectsInvalidField(t *testing.T) {
	h, cleanup := setupFindingsHandler(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/findings/autocomplete?field=bad", nil)
	req = makeSessionContext(req, "u1", 1, []string{"superadmin"})
	rr := httptest.NewRecorder()
	h.Autocomplete(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "findings.autocomplete.fieldInvalid") {
		t.Fatalf("expected findings.autocomplete.fieldInvalid, got %q", rr.Body.String())
	}
}
