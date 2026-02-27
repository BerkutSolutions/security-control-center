package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestRequiredMenuKeysCoversShellTabsAndAPI(t *testing.T) {
	s := &Server{}
	cases := []struct {
		path string
		want []string
	}{
		{path: "/dashboard", want: []string{"dashboard"}},
		{path: "/api/dashboard/stats", want: []string{"dashboard"}},

		{path: "/tasks", want: []string{"tasks"}},
		{path: "/api/tasks/space/1", want: []string{"tasks"}},

		{path: "/docs", want: []string{"docs"}},
		{path: "/api/docs/1", want: []string{"docs"}},

		{path: "/approvals", want: []string{"approvals"}},
		{path: "/api/approvals/1", want: []string{"approvals"}},

		{path: "/incidents", want: []string{"incidents"}},
		{path: "/api/incidents/1", want: []string{"incidents"}},

		{path: "/controls", want: []string{"controls"}},
		{path: "/registry", want: []string{"controls"}},
		{path: "/api/controls/1", want: []string{"controls"}},

		{path: "/assets", want: []string{"controls", "assets"}},
		{path: "/api/assets/1", want: []string{"controls", "assets"}},
		{path: "/api/assets/export.csv", want: []string{"controls", "assets"}},
		{path: "/api/assets/autocomplete", want: []string{"controls", "assets"}},
		{path: "/software", want: []string{"controls", "software"}},
		{path: "/api/software/1", want: []string{"controls", "software"}},

		{path: "/monitoring", want: []string{"monitoring"}},
		{path: "/api/monitoring/events", want: []string{"monitoring"}},

		{path: "/reports", want: []string{"reports"}},
		{path: "/api/reports/1/charts", want: []string{"reports"}},

		{path: "/backups", want: []string{"backups"}},
		{path: "/api/backups/list", want: []string{"backups"}},

		{path: "/findings", want: []string{"controls", "findings"}},
		{path: "/api/findings/list", want: []string{"controls", "findings"}},
		{path: "/api/findings/export.csv", want: []string{"controls", "findings"}},
		{path: "/api/findings/autocomplete", want: []string{"controls", "findings"}},

		{path: "/accounts", want: []string{"accounts"}},
		{path: "/api/accounts/users", want: []string{"accounts"}},

		{path: "/settings", want: []string{"settings"}},
		{path: "/api/settings/runtime", want: []string{"settings"}},

		{path: "/logs", want: []string{"logs"}},
		{path: "/api/logs", want: []string{"logs"}},

		// Special cases: should never be menu-restricted.
		{path: "/api/auth/login", want: nil},
		{path: "/api/app/meta", want: nil},
		{path: "/api/page/entry", want: nil},
		{path: "/static/app.css", want: nil},
		{path: "/", want: nil},
		{path: "/app", want: nil},
	}

	for _, tc := range cases {
		got := s.requiredMenuKeys(tc.path)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("requiredMenuKeys(%q)=%v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestMenuPermissionsDenyTabEvenWithRolePermission(t *testing.T) {
	users, groups := newTestUserAndGroupStores(t)

	// Admin role is allowed almost everywhere; menu guard should still apply.
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	s := &Server{policy: policy, users: users}

	ctx := context.Background()
	uid, _ := users.Create(ctx, &store.User{Username: "u1", Active: true}, []string{"admin"})

	// Restrict user to only "reports" via group menu_permissions.
	_, err := groups.Create(ctx, &store.Group{Name: "g1", MenuPermissions: []string{"reports"}}, nil, []int64{uid})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	okHandler := s.requirePermission("monitoring.view")(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/events", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   uid,
		Username: "u1",
		Roles:    []string{"admin"},
	}))
	rr := httptest.NewRecorder()
	okHandler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden due to menu_permissions, got %d", rr.Code)
	}

	allowedHandler := s.requirePermission("reports.view")(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req2 := httptest.NewRequest(http.MethodGet, "/api/reports/1/charts", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   uid,
		Username: "u1",
		Roles:    []string{"admin"},
	}))
	rr2 := httptest.NewRecorder()
	allowedHandler(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected ok for allowed tab, got %d", rr2.Code)
	}
}

func newTestUserAndGroupStores(t *testing.T) (store.UsersStore, store.GroupsStore) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "menu_guard.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return store.NewUsersStore(db), store.NewGroupsStore(db)
}
