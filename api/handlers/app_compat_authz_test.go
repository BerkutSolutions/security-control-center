package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

func TestAppCompatReportForbiddenWithoutPermission(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy([]rbac.Role{{Name: "r0", Permissions: nil}})
	h := NewAppCompatHandler(store.NewAppModuleStateStore(db), policy)

	req := httptest.NewRequest(http.MethodGet, "/api/app/compat", nil)
	rr := httptest.NewRecorder()
	h.Report(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rr.Code, rr.Body.String())
	}
}

