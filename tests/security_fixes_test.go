package tests

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func TestLegacyImportDisabledByDefault(t *testing.T) {
	acc, users, _, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	actor := &store.User{Username: "actor", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	actorID, _ := users.Create(ctx, actor, []string{"admin"})

	csvBody := "username,full_name,role\nsa,Super Admin,superadmin"
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", "users.csv")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write([]byte(csvBody)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/import", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = makeSessionContext(req, actor.Username, actorID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.ImportUsers(rr, req)
	if rr.Code != http.StatusGone {
		t.Fatalf("expected 410, got %d", rr.Code)
	}
	u, _, _ := users.FindByUsername(ctx, "sa")
	if u != nil {
		t.Fatalf("unexpected user created for disabled import")
	}
}

func TestGroupReplacementProtectsLastSuperadmin(t *testing.T) {
	acc, users, groups, _, cfg, cleanup := setupImportHandler(t)
	defer cleanup()
	ctx := context.Background()
	ph := auth.MustHashPassword("Password123!", cfg.Pepper)
	user := &store.User{Username: "last", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	userID, _ := users.Create(ctx, user, nil)
	groupID, err := groups.Create(ctx, &store.Group{Name: "sa-group"}, []string{"superadmin"}, nil)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := groups.SetUserGroups(ctx, userID, []int64{groupID}); err != nil {
		t.Fatalf("set user groups: %v", err)
	}

	body := bytes.NewReader([]byte(`{"groups":[]}`))
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/users/"+strconv.FormatInt(userID, 10), body)
	req = makeSessionContext(req, "last", userID, []string{"superadmin"})
	rr := httptest.NewRecorder()
	acc.UpdateUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}

func TestSelfLockDenied(t *testing.T) {
	acc, _, users := newTestAccountsHandler(t)
	ph := auth.MustHashPassword("p", "pepper")
	user := &store.User{Username: "selflock", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	userID, _ := users.Create(context.Background(), user, []string{"admin"})
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/users/"+strconv.FormatInt(userID, 10)+"/lock", bytes.NewReader([]byte(`{}`)))
	req = makeSessionContext(req, "selflock", userID, []string{"admin"})
	rr := httptest.NewRecorder()
	acc.LockUser(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rr.Code)
	}
}
