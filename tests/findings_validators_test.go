package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func TestFindingsValidatorsRejectInvalidEnums(t *testing.T) {
	h, cleanup := setupFindingsHandler(t)
	defer cleanup()

	makeReq := func(body any) *http.Request {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/findings", bytes.NewReader(raw))
		req = makeSessionContext(req, "u1", 1, []string{"superadmin"})
		return req
	}

	rr := httptest.NewRecorder()
	h.Create(rr, makeReq(map[string]any{"title": "t1", "status": "bad"}))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("findings.statusInvalid")) {
		t.Fatalf("expected findings.statusInvalid, got %q", rr.Body.String())
	}

	rr2 := httptest.NewRecorder()
	h.Create(rr2, makeReq(map[string]any{"title": "t1", "severity": "bad"}))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr2.Code)
	}
	if !bytes.Contains(rr2.Body.Bytes(), []byte("findings.severityInvalid")) {
		t.Fatalf("expected findings.severityInvalid, got %q", rr2.Body.String())
	}

	rr3 := httptest.NewRecorder()
	h.Create(rr3, makeReq(map[string]any{"title": "t1", "finding_type": "bad"}))
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr3.Code)
	}
	if !bytes.Contains(rr3.Body.Bytes(), []byte("findings.typeInvalid")) {
		t.Fatalf("expected findings.typeInvalid, got %q", rr3.Body.String())
	}
}

func TestFindingsValidatorsRejectInvalidEnumsOnUpdate(t *testing.T) {
	h, fs, cleanup := setupFindingsHandlerWithStore(t)
	defer cleanup()

	id, err := fs.CreateFinding(context.Background(), &store.Finding{Title: "ok", Status: "open", Severity: "low", FindingType: "other"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"title":        "ok2",
		"status":       "bad",
		"severity":     "low",
		"finding_type": "other",
		"version":      1,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/findings/1", bytes.NewReader(raw))
	req = withURLParams(req, map[string]string{"id": itoa(id)})
	req = makeSessionContext(req, "u1", 1, []string{"superadmin"})
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("findings.statusInvalid")) {
		t.Fatalf("expected findings.statusInvalid, got %q", rr.Body.String())
	}
}

func setupFindingsHandler(t *testing.T) (*handlers.FindingsHandler, func()) {
	t.Helper()
	h, _, cleanup := setupFindingsHandlerWithStore(t)
	return h, cleanup
}

func setupFindingsHandlerWithStore(t *testing.T) (*handlers.FindingsHandler, store.FindingsStore, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "findings.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	fs := store.NewFindingsStore(db)
	h := handlers.NewFindingsHandler(fs, nil, nil, nil, nil, nil, store.NewAuditStore(db), policy)
	return h, fs, func() { _ = db.Close() }
}
