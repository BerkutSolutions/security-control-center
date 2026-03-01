package handlers

import (
	"encoding/json"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/appcompat"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

func TestAppCompatReportReturnsModules(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := NewAppCompatHandler(store.NewAppModuleStateStore(db), policy)

	req := httptest.NewRequest(http.MethodGet, "/api/app/compat", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"admin"},
	}))
	rr := httptest.NewRecorder()
	h.Report(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var report appcompat.Report
	if err := json.NewDecoder(rr.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.Items) == 0 {
		t.Fatalf("expected non-empty items")
	}
}
