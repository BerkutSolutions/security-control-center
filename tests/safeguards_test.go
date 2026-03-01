package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type safeguardsMockSessions struct{ killed int }

func (m *safeguardsMockSessions) SaveSession(ctx context.Context, sess *store.SessionRecord) error {
	return nil
}
func (m *safeguardsMockSessions) GetSession(ctx context.Context, id string) (*store.SessionRecord, error) {
	return nil, nil
}
func (m *safeguardsMockSessions) ListByUser(ctx context.Context, userID int64) ([]store.SessionRecord, error) {
	return nil, nil
}
func (m *safeguardsMockSessions) ListAll(ctx context.Context) ([]store.SessionRecord, error) {
	return nil, nil
}
func (m *safeguardsMockSessions) DeleteSession(ctx context.Context, id string, by string) error {
	return nil
}
func (m *safeguardsMockSessions) DeleteAllForUser(ctx context.Context, userID int64, by string) error {
	m.killed++
	return nil
}
func (m *safeguardsMockSessions) DeleteAll(ctx context.Context, by string) error { return nil }
func (m *safeguardsMockSessions) UpdateActivity(ctx context.Context, id string, now time.Time, extendBy time.Duration) error {
	return nil
}

func newTestAccountsHandler(t *testing.T) (*handlers.AccountsHandler, *safeguardsMockSessions, store.UsersStore) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "safeguard.db"), Pepper: "pepper", Security: config.SecurityConfig{TagsSubsetEnforced: true}}
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
	sessionsStore := &safeguardsMockSessions{}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	acc := handlers.NewAccountsHandler(users, nil, nil, sessionsStore, store.NewAuth2FAStore(db), policy, auth.NewSessionManager(sessionsStore, cfg, logger), cfg, store.NewAuditStore(db), logger, nil)
	return acc, sessionsStore, users
}

func makeSessionContextSafeguards(r *http.Request, username string, userID int64, roles []string) *http.Request {
	rec := &store.SessionRecord{Username: username, UserID: userID, Roles: roles}
	ctx := context.WithValue(r.Context(), auth.SessionContextKey, rec)
	return r.WithContext(ctx)
}

func TestClearanceCap(t *testing.T) {
	acc, _, users := newTestAccountsHandler(t)
	actorHash := auth.MustHashPassword("p", "pepper")
	actor := &store.User{Username: "admin1", PasswordHash: actorHash.Hash, Salt: actorHash.Salt, PasswordSet: true, Active: true, ClearanceLevel: 1}
	actorID, _ := users.Create(context.Background(), actor, []string{"admin"})
	target := &store.User{Username: "user1", PasswordHash: actorHash.Hash, Salt: actorHash.Salt, PasswordSet: true, Active: true, ClearanceLevel: 1}
	targetID, _ := users.Create(context.Background(), target, []string{"admin"})
	body, _ := json.Marshal(map[string]interface{}{"clearance_level": 3})
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(targetID, 10), bytes.NewReader(body))
	req = makeSessionContextSafeguards(req, "admin1", actorID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestTagsSubsetEnforced(t *testing.T) {
	acc, _, users := newTestAccountsHandler(t)
	ph := auth.MustHashPassword("p", "pepper")
	actor := &store.User{Username: "admin2", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceTags: []string{"a"}}
	actorID, _ := users.Create(context.Background(), actor, []string{"admin"})
	target := &store.User{Username: "user2", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	targetID, _ := users.Create(context.Background(), target, []string{"admin"})
	body, _ := json.Marshal(map[string]interface{}{"clearance_tags": []string{"b"}})
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(targetID, 10), bytes.NewReader(body))
	req = makeSessionContextSafeguards(req, "admin2", actorID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestLastSuperadminProtected(t *testing.T) {
	acc, _, users := newTestAccountsHandler(t)
	ph := auth.MustHashPassword("p", "pepper")
	sup := &store.User{Username: "sa", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	supID, _ := users.Create(context.Background(), sup, []string{"superadmin"})
	body, _ := json.Marshal(map[string]interface{}{"status": "disabled"})
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(supID, 10), bytes.NewReader(body))
	req = makeSessionContextSafeguards(req, "sa", supID, []string{"superadmin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSelfLockoutPrevented(t *testing.T) {
	acc, _, users := newTestAccountsHandler(t)
	ph := auth.MustHashPassword("p", "pepper")
	admin := &store.User{Username: "self", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	adminID, _ := users.Create(context.Background(), admin, []string{"admin"})
	body, _ := json.Marshal(map[string]interface{}{"roles": []string{"doc_viewer"}})
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(adminID, 10), bytes.NewReader(body))
	req = makeSessionContextSafeguards(req, "self", adminID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSessionsKilledOnChange(t *testing.T) {
	acc, sessMock, users := newTestAccountsHandler(t)
	ph := auth.MustHashPassword("p", "pepper")
	admin := &store.User{Username: "a1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	adminID, _ := users.Create(context.Background(), admin, []string{"admin"})
	target := &store.User{Username: "t1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	targetID, _ := users.Create(context.Background(), target, []string{"admin"})
	body, _ := json.Marshal(map[string]interface{}{"roles": []string{"doc_viewer"}})
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(targetID, 10), bytes.NewReader(body))
	req = makeSessionContextSafeguards(req, "a1", adminID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if sessMock.killed == 0 {
		t.Fatalf("expected sessions killed")
	}
}
