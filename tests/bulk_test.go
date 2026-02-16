package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type bulkSessionsMock struct{ killed map[int64]int }

func (m *bulkSessionsMock) SaveSession(ctx context.Context, sess *store.SessionRecord) error {
	return nil
}
func (m *bulkSessionsMock) GetSession(ctx context.Context, id string) (*store.SessionRecord, error) {
	return nil, nil
}
func (m *bulkSessionsMock) ListByUser(ctx context.Context, userID int64) ([]store.SessionRecord, error) {
	return nil, nil
}
func (m *bulkSessionsMock) ListAll(ctx context.Context) ([]store.SessionRecord, error) {
	return nil, nil
}
func (m *bulkSessionsMock) DeleteSession(ctx context.Context, id string, by string) error { return nil }
func (m *bulkSessionsMock) DeleteAllForUser(ctx context.Context, userID int64, by string) error {
	if m.killed == nil {
		m.killed = map[int64]int{}
	}
	m.killed[userID]++
	return nil
}
func (m *bulkSessionsMock) DeleteAll(ctx context.Context, by string) error { return nil }
func (m *bulkSessionsMock) UpdateActivity(ctx context.Context, id string, now time.Time, extendBy time.Duration) error {
	return nil
}

func setupBulkHandler(t *testing.T) (*handlers.AccountsHandler, store.UsersStore, *bulkSessionsMock, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "bulk.db"), Pepper: "pepper"}
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
	groups := store.NewGroupsStore(db)
	roles := store.NewRolesStore(db)
	if err := roles.EnsureBuiltIn(context.Background(), convertRoles(rbac.DefaultRoles())); err != nil {
		t.Fatalf("roles: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	sessions := &bulkSessionsMock{killed: map[int64]int{}}
	acc := handlers.NewAccountsHandler(users, groups, roles, sessions, policy, auth.NewSessionManager(sessions, cfg, logger), cfg, store.NewAuditStore(db), logger, nil)
	return acc, users, sessions, func() { db.Close() }
}

func TestBulkAssignRolePartialSuccess(t *testing.T) {
	acc, users, sessions, cleanup := setupBulkHandler(t)
	defer cleanup()
	ph := auth.MustHashPassword("passWORD123!", "pepper")
	admin := &store.User{Username: "admin1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	adminID, _ := users.Create(context.Background(), admin, []string{"admin"})
	target := &store.User{Username: "u1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	targetID, _ := users.Create(context.Background(), target, []string{})

	body, _ := json.Marshal(map[string]any{
		"action":   "assign_role",
		"user_ids": []int64{targetID, targetID + 123},
		"payload":  map[string]any{"role_id": "doc_viewer"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/users/bulk", bytes.NewReader(body))
	req = makeSessionContext(req, "admin1", adminID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.BulkUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Success int `json:"success_count"`
		Failed  int `json:"failed_count"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Success != 1 || resp.Failed != 1 {
		t.Fatalf("unexpected counts: %+v", resp)
	}
	_, roles, _ := users.Get(context.Background(), targetID)
	if !sliceContains(roles, "doc_viewer") {
		t.Fatalf("role not assigned: %v", roles)
	}
	if sessions.killed[targetID] != 1 {
		t.Fatalf("sessions not killed for target")
	}
}

func TestBulkLockProtectsLastSuperadmin(t *testing.T) {
	acc, users, _, cleanup := setupBulkHandler(t)
	defer cleanup()
	ph := auth.MustHashPassword("passWORD123!", "pepper")
	super := &store.User{Username: "sa1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	superID, _ := users.Create(context.Background(), super, []string{"superadmin"})
	body, _ := json.Marshal(map[string]any{
		"action":   "lock",
		"user_ids": []int64{superID},
		"payload":  map[string]any{"reason": "test"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/users/bulk", bytes.NewReader(body))
	req = makeSessionContext(req, "sa1", superID, []string{"superadmin"})
	rr := httptest.NewRecorder()
	acc.BulkUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Success  int              `json:"success_count"`
		Failed   int              `json:"failed_count"`
		Failures []map[string]any `json:"failures"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Success != 0 || resp.Failed != 1 {
		t.Fatalf("expected safeguard failure, got %+v", resp)
	}
	if len(resp.Failures) == 0 || resp.Failures[0]["reason"] != "last_superadmin" {
		t.Fatalf("expected last_superadmin failure, got %+v", resp.Failures)
	}
	u, _, _ := users.Get(context.Background(), superID)
	if u.LockStage != 0 {
		t.Fatalf("superadmin should not be locked")
	}
}

func TestBulkResetPasswordGeneratesAndKillsSessions(t *testing.T) {
	acc, users, sessions, cleanup := setupBulkHandler(t)
	defer cleanup()
	ph := auth.MustHashPassword("passWORD123!", "pepper")
	admin := &store.User{Username: "admin2", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	adminID, _ := users.Create(context.Background(), admin, []string{"admin"})
	target := &store.User{Username: "reset-me", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	targetID, _ := users.Create(context.Background(), target, []string{})

	body, _ := json.Marshal(map[string]any{
		"action":   "reset_password",
		"user_ids": []int64{targetID},
		"payload":  map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/users/bulk", bytes.NewReader(body))
	req = makeSessionContext(req, "admin2", adminID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.BulkUsers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Success   int                      `json:"success_count"`
		Passwords []map[string]interface{} `json:"passwords"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Success != 1 || len(resp.Passwords) != 1 {
		t.Fatalf("unexpected password response %+v", resp)
	}
	u, _, _ := users.Get(context.Background(), targetID)
	if !u.RequirePasswordChange {
		t.Fatalf("require_password_change not set")
	}
	if sessions.killed[targetID] != 1 {
		t.Fatalf("sessions not killed after reset")
	}
}

func TestBulkRequiresPermission(t *testing.T) {
	acc, users, _, cleanup := setupBulkHandler(t)
	defer cleanup()
	ph := auth.MustHashPassword("passWORD123!", "pepper")
	user := &store.User{Username: "simple", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, _ := users.Create(context.Background(), user, []string{"doc_viewer"})
	body, _ := json.Marshal(map[string]any{
		"action":   "disable",
		"user_ids": []int64{uid},
		"payload":  map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/users/bulk", bytes.NewReader(body))
	req = makeSessionContext(req, "simple", uid, []string{"doc_viewer"})
	rr := httptest.NewRecorder()
	acc.BulkUsers(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
