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

func TestAppJobsCreateRequiresSessionUser(t *testing.T) {
	db := mustTestDB(t)
	h := NewAppJobsHandler(store.NewAppJobsStore(db), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"all","mode":"partial"}`))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAppJobsCreateAndGet(t *testing.T) {
	db := mustTestDB(t)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := NewAppJobsHandler(store.NewAppJobsStore(db), policy)

	req := httptest.NewRequest(http.MethodPost, "/api/app/jobs", strings.NewReader(`{"type":"reinit","scope":"module","module_id":"monitoring","mode":"partial"}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{
		UserID:   1,
		Username: "u1",
		Roles:    []string{"admin"},
	}))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rr.Code, rr.Body.String())
	}
}
