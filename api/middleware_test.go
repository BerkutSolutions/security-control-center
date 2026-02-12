package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

func TestRequirePermissionDeniesMissingPermission(t *testing.T) {
	s := &Server{policy: rbac.NewPolicy(rbac.DefaultRoles())}
	handler := s.requirePermission("reports.edit")(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/reports/1/charts", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		Username: "manager",
		Roles:    []string{"manager"},
	}))
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rr.Code)
	}
}
