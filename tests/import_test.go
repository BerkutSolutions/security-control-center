package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type importResponse struct {
	ImportID        string     `json:"import_id"`
	DetectedHeaders []string   `json:"detected_headers"`
	PreviewRows     [][]string `json:"preview_rows"`
}

type importCommitResponse struct {
	TotalRows    int `json:"total_rows"`
	CreatedCount int `json:"created_count"`
	UpdatedCount int `json:"updated_count"`
	FailedCount  int `json:"failed_count"`
	Failures     []struct {
		RowNumber int    `json:"row_number"`
		Reason    string `json:"reason"`
		Detail    string `json:"detail"`
	} `json:"failures"`
	CreatedUsers []struct {
		Login        string `json:"login"`
		TempPassword string `json:"temp_password"`
	} `json:"created_users"`
}

func TestImportCreatesUsersWithMapping(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", FullName: "Actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceLevel: 5}
	actorID, _ := users.Create(ctx, actor, []string{"superadmin"})
	csvBody := "login,full_name,roles,department,status\nuser1,User One,admin,IT,active\nuser2,User Two,doc_viewer,HR,disabled"
	importID := uploadCSV(t, acc, actor.Username, actorID, csvBody)
	mapping := map[string]string{
		"login":      "login",
		"full_name":  "full_name",
		"roles":      "roles",
		"department": "department",
		"status":     "status",
	}
	res := commitImport(t, acc, actor.Username, actorID, mapping, map[string]any{"import_id": importID})
	if res.CreatedCount != 2 || res.FailedCount != 0 {
		t.Fatalf("unexpected import result: %+v", res)
	}
	u1, _, _ := users.FindByUsername(ctx, "user1")
	if u1 == nil || !u1.PasswordSet || !u1.RequirePasswordChange {
		t.Fatalf("user1 not created with temp password flags: %+v", u1)
	}
	u2, _, _ := users.FindByUsername(ctx, "user2")
	if u2 == nil || u2.Active {
		t.Fatalf("user2 should be disabled")
	}
	if len(res.CreatedUsers) != 2 || res.CreatedUsers[0].TempPassword == "" {
		t.Fatalf("temp passwords not returned: %+v", res.CreatedUsers)
	}
}

func TestImportAlreadyExistsFails(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceLevel: 2}
	actorID, _ := users.Create(ctx, actor, []string{"superadmin"})
	existing := &store.User{Username: "dupuser", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	_, _ = users.Create(ctx, existing, []string{"admin"})
	csvBody := "login,full_name,roles\ndupuser,User One,admin"
	importID := uploadCSV(t, acc, actor.Username, actorID, csvBody)
	mapping := map[string]string{"login": "login", "full_name": "full_name", "roles": "roles"}
	res := commitImport(t, acc, actor.Username, actorID, mapping, map[string]any{"import_id": importID})
	if res.CreatedCount != 0 || res.FailedCount != 1 {
		t.Fatalf("expected failure, got %+v", res)
	}
	if len(res.Failures) == 0 {
		t.Fatalf("expected failures details")
	}
	if res.Failures[0].Reason != "already_exists" {
		t.Fatalf("expected already_exists, got %s", res.Failures[0].Reason)
	}
}

func TestImportRoleAndGroupValidation(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceLevel: 3}
	actorID, _ := users.Create(ctx, actor, []string{"superadmin"})
	csvBody := "login,full_name,roles,groups\nruser,Role Missing,missing_role,\nguser,Group Missing,admin,missing_group"
	importID := uploadCSV(t, acc, actor.Username, actorID, csvBody)
	mapping := map[string]string{"login": "login", "full_name": "full_name", "roles": "roles", "groups": "groups"}
	res := commitImport(t, acc, actor.Username, actorID, mapping, map[string]any{"import_id": importID})
	if res.FailedCount != 2 {
		t.Fatalf("expected two failures, got %+v", res)
	}
	reasons := []string{res.Failures[0].Reason, res.Failures[1].Reason}
	if !contains(reasons, "role_not_found") || !contains(reasons, "group_not_found") {
		t.Fatalf("unexpected reasons: %v", reasons)
	}
}

func TestImportClearanceCap(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceLevel: 1}
	actorID, _ := users.Create(ctx, actor, []string{"superadmin"})
	csvBody := "login,full_name,clearance_level,roles\nsecure,Secure User,5,admin"
	importID := uploadCSV(t, acc, actor.Username, actorID, csvBody)
	mapping := map[string]string{"login": "login", "full_name": "full_name", "clearance_level": "clearance_level", "roles": "roles"}
	res := commitImport(t, acc, actor.Username, actorID, mapping, map[string]any{"import_id": importID})
	if res.CreatedCount != 0 || res.FailedCount != 1 {
		t.Fatalf("expected failure, got %+v", res)
	}
	if len(res.Failures) == 0 || res.Failures[0].Reason != "clearance_too_high" {
		t.Fatalf("expected clearance_too_high, got %s", res.Failures[0].Reason)
	}
}

func TestImportTempPasswordMustChange(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, ClearanceLevel: 3}
	actorID, _ := users.Create(ctx, actor, []string{"superadmin"})
	csvBody := "login,full_name,roles\nneedpwd,Needs Password,admin"
	importID := uploadCSV(t, acc, actor.Username, actorID, csvBody)
	mapping := map[string]string{"login": "login", "full_name": "full_name", "roles": "roles"}
	res := commitImport(t, acc, actor.Username, actorID, mapping, map[string]any{
		"import_id": importID,
		"options":   map[string]any{"temp_password": true, "must_change_password": true},
	})
	if res.CreatedCount != 1 || len(res.CreatedUsers) != 1 || res.CreatedUsers[0].TempPassword == "" {
		t.Fatalf("expected temp password generated, got %+v", res)
	}
	u, _, _ := users.FindByUsername(ctx, "needpwd")
	if u == nil || !u.RequirePasswordChange {
		t.Fatalf("must_change_password not set")
	}
}

func uploadCSV(t *testing.T, acc *handlers.AccountsHandler, username string, userID int64, content string) string {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", "users.csv")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	writer.Close()
	req := httptest.NewRequest("POST", "/api/accounts/import/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = makeSessionContext(req, username, userID, []string{"superadmin"})
	rr := httptest.NewRecorder()
	acc.ImportUpload(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("upload status %d: %s", rr.Code, rr.Body.String())
	}
	var resp importResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ImportID == "" {
		t.Fatalf("missing import id: %+v", resp)
	}
	return resp.ImportID
}

func commitImport(t *testing.T, acc *handlers.AccountsHandler, username string, userID int64, mapping map[string]string, extra map[string]any) importCommitResponse {
	t.Helper()
	payload := map[string]any{
		"import_id": "",
		"mapping":   mapping,
		"options":   map[string]any{"temp_password": true, "must_change_password": true},
		"defaults":  map[string]any{},
	}
	for k, v := range extra {
		payload[k] = v
	}
	id := fmt.Sprint(payload["import_id"])
	if id == "" {
		t.Fatalf("import_id required in payload")
	}
	payload["import_id"] = id
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/accounts/import/commit", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = makeSessionContext(req, username, userID, []string{"superadmin"})
	rr := httptest.NewRecorder()
	acc.ImportCommit(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("commit status %d: %s", rr.Code, rr.Body.String())
	}
	var resp importCommitResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode commit: %v", err)
	}
	return resp
}

func setupImportHandler(t *testing.T) (*handlers.AccountsHandler, store.UsersStore, store.GroupsStore, store.RolesStore, *config.AppConfig, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "import.db"), Pepper: "pepper"}
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
		t.Fatalf("ensure roles: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	sessions := &mockSessions{}
	audits := store.NewAuditStore(db)
	acc := handlers.NewAccountsHandler(users, groups, roles, sessions, policy, auth.NewSessionManager(sessions, cfg, logger), cfg, audits, logger, nil)
	return acc, users, groups, roles, cfg, func() { db.Close() }
}

func contains(list []string, target string) bool {
	for _, v := range list {
		if strings.EqualFold(v, target) {
			return true
		}
	}
	return false
}
