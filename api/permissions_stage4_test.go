package api

import (
	"context"
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

func TestNewStage4EndpointsRequirePermission(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "stage4_perms.db"), Pepper: "pepper"}
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
	uid, _ := users.Create(context.Background(), &store.User{Username: "u1", Active: true}, nil)

	policy := rbac.NewPolicy(rbac.DefaultRoles())
	s := &Server{policy: policy, users: users}

	okHandler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

	cases := []struct {
		path string
		perm string
	}{
		{path: "/api/assets/export.csv", perm: "assets.view"},
		{path: "/api/assets/autocomplete", perm: "assets.view"},
		{path: "/api/findings/export.csv", perm: "findings.view"},
		{path: "/api/findings/autocomplete", perm: "findings.view"},
	}

	for _, tc := range cases {
		handler := s.requirePermission(rbac.Permission(tc.perm))(okHandler)
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
			UserID:   uid,
			Username: "u1",
			Roles:    []string{"manager"},
		}))
		rr := httptest.NewRecorder()
		handler(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected forbidden for %s without %s, got %d", tc.path, tc.perm, rr.Code)
		}
	}
}
