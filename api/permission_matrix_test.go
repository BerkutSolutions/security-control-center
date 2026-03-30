package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type permissionMatrixCase struct {
	name  string
	path  string
	kind  string // "all" | "any"
	perms []rbac.Permission
}

func TestPermissionMatrixZeroTrustForDirectAPI(t *testing.T) {
	s := &Server{policy: rbac.NewPolicy(rbac.DefaultRoles())}
	roleNames := defaultRoleNames()

	cases := []permissionMatrixCase{
		{name: "dashboard-read", path: "/api/dashboard", kind: "all", perms: []rbac.Permission{"dashboard.view"}},
		{name: "accounts-read", path: "/api/accounts/users", kind: "all", perms: []rbac.Permission{"accounts.view"}},
		{name: "docs-read", path: "/api/docs", kind: "all", perms: []rbac.Permission{"docs.view"}},
		{name: "approvals-read", path: "/api/approvals", kind: "all", perms: []rbac.Permission{"approvals.view"}},
		{name: "incidents-read", path: "/api/incidents", kind: "all", perms: []rbac.Permission{"incidents.view"}},
		{name: "tasks-read", path: "/api/tasks", kind: "all", perms: []rbac.Permission{"tasks.view"}},
		{name: "monitoring-read", path: "/api/monitoring/monitors", kind: "all", perms: []rbac.Permission{"monitoring.view"}},
		{name: "backups-read", path: "/api/backups", kind: "all", perms: []rbac.Permission{"backups.read"}},
		{name: "logs-read", path: "/api/logs", kind: "all", perms: []rbac.Permission{"logs.view"}},
		{name: "accesses-read-any", path: "/api/accesses", kind: "any", perms: []rbac.Permission{"accounts.view", "accounts.manage", "app.view"}},
		{name: "services-read-any", path: "/api/services", kind: "any", perms: []rbac.Permission{"accounts.view", "accounts.manage", "settings.tags", "app.view"}},
		{name: "notifications-settings-read-any", path: "/api/notifications/settings", kind: "any", perms: []rbac.Permission{"monitoring.notifications.view", "monitoring.settings.manage", "accounts.view"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// For each known role: allow only when role has required permission(s), otherwise deny with 403.
			for _, role := range roleNames {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
					Username: role + "-user",
					Roles:    []string{role},
				}))
				rr := httptest.NewRecorder()
				wrapCaseHandler(s, tc)(rr, req)

				allowed := roleAllowed(s.policy, role, tc)
				if allowed && rr.Code != http.StatusNoContent {
					t.Fatalf("role %q should be allowed for %s, got %d", role, tc.path, rr.Code)
				}
				if !allowed && rr.Code != http.StatusForbidden {
					t.Fatalf("role %q should be forbidden for %s, got %d", role, tc.path, rr.Code)
				}
			}
		})
	}
}

func wrapCaseHandler(s *Server, tc permissionMatrixCase) http.HandlerFunc {
	terminal := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	switch tc.kind {
	case "any":
		return s.requireAnyPermission(tc.perms...)(terminal)
	default:
		return s.requirePermission(tc.perms[0])(terminal)
	}
}

func roleAllowed(policy *rbac.Policy, role string, tc permissionMatrixCase) bool {
	switch tc.kind {
	case "any":
		for _, perm := range tc.perms {
			if policy.Allowed([]string{role}, perm) {
				return true
			}
		}
		return false
	default:
		return policy.Allowed([]string{role}, tc.perms[0])
	}
}

func defaultRoleNames() []string {
	def := rbac.DefaultRoles()
	out := make([]string, 0, len(def))
	seen := map[string]struct{}{}
	for _, r := range def {
		name := strings.ToLower(strings.TrimSpace(r.Name))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
