package handlers

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

func TestAppJobsCreateForbiddenWithoutCompatManagePartial(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "r0", Permissions: nil},
	})
	h := NewAppJobsHandler(store.NewAppJobsStore(db), policy)

	req := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"monitoring","mode":"partial"}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"r0"},
	}))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAppJobsCreateForbiddenWithoutSettingsAdvanced(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "r1", Permissions: []rbac.Permission{"app.compat.manage.partial", "monitoring.manage"}},
	})
	h := NewAppJobsHandler(store.NewAppJobsStore(db), policy)

	req := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"monitoring","mode":"partial"}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"r1"},
	}))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAppJobsCreateFullResetCriticalModuleRequiresSuperadminRole(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy([]rbac.Role{
		{Name: "admin", Permissions: []rbac.Permission{
			"app.compat.manage.partial",
			"app.compat.manage.full",
			"settings.advanced",
			"docs.manage",
		}},
		{Name: "superadmin", Permissions: []rbac.Permission{
			"app.compat.manage.partial",
			"app.compat.manage.full",
			"settings.advanced",
			"docs.manage",
		}},
	})
	h := NewAppJobsHandler(store.NewAppJobsStore(db), policy)

	// admin role has all permissions, but still blocked because module is critical and role is not superadmin.
	req := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"docs","mode":"full"}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"admin"},
	}))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin, got %d (%s)", rr.Code, rr.Body.String())
	}

	// superadmin allowed.
	req2 := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"docs","mode":"full"}`))
	req2 = req2.WithContext(context.WithValue(req2.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   2,
		Username: "u2",
		Roles:    []string{"superadmin"},
	}))
	rr2 := httptest.NewRecorder()
	h.Create(rr2, req2)
	if rr2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for superadmin, got %d (%s)", rr2.Code, rr2.Body.String())
	}
}

