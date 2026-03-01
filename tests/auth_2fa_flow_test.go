package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func Test2FALoginFlowTOTP(t *testing.T) {
	sessions, users, _, authHandler, cfg, _, _, _, _, cleanup := setupSessionEnv(t)
	defer cleanup()
	_ = sessions

	ctx := context.Background()
	pass := "S3cure#Pass"
	ph := auth.MustHashPassword(pass, cfg.Pepper)
	u := &store.User{Username: "alice", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, err := users.Create(ctx, u, []string{"admin"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("secret: %v", err)
	}
	secretEnc, err := auth.EncryptTOTPSecret(secret, cfg.Pepper)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := users.SetTOTP(ctx, uid, true, secretEnc); err != nil {
		t.Fatalf("set totp: %v", err)
	}

	loginBody := map[string]any{"username": "alice", "password": pass}
	rawLogin, _ := json.Marshal(loginBody)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(rawLogin))
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()
	authHandler.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("login expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var loginResp struct {
		TwoFARequired bool   `json:"two_factor_required"`
		ChallengeID   string `json:"challenge_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login resp: %v", err)
	}
	if !loginResp.TwoFARequired || loginResp.ChallengeID == "" {
		t.Fatalf("expected 2fa challenge, got: %+v", loginResp)
	}

	code, err := auth.ComputeTOTPCode(secret, time.Now().UTC(), auth.DefaultTOTPConfig())
	if err != nil {
		t.Fatalf("totp code: %v", err)
	}
	raw2fa, _ := json.Marshal(map[string]any{"challenge_id": loginResp.ChallengeID, "code": code})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login/2fa", bytes.NewReader(raw2fa))
	req2.RemoteAddr = "10.0.0.1:1234"
	req2.Header.Set("User-Agent", "test-agent")
	rr2 := httptest.NewRecorder()
	authHandler.Login2FA(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("login2fa expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}

	foundSessionCookie := false
	for _, c := range rr2.Result().Cookies() {
		if c.Name == "berkut_session" && c.Value != "" {
			foundSessionCookie = true
			break
		}
	}
	if !foundSessionCookie {
		t.Fatalf("expected session cookie on 2fa login")
	}
}

func Test2FALoginInvalidCodeAudited(t *testing.T) {
	_, users, _, authHandler, cfg, db, _, _, _, cleanup := setupSessionEnv(t)
	defer cleanup()

	ctx := context.Background()
	pass := "S3cure#Pass"
	ph := auth.MustHashPassword(pass, cfg.Pepper)
	u := &store.User{Username: "alice", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, err := users.Create(ctx, u, []string{"admin"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	secret, _ := auth.GenerateTOTPSecret()
	secretEnc, _ := auth.EncryptTOTPSecret(secret, cfg.Pepper)
	_ = users.SetTOTP(ctx, uid, true, secretEnc)

	rawLogin, _ := json.Marshal(map[string]any{"username": "alice", "password": pass})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(rawLogin))
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()
	authHandler.Login(rr, req)

	var loginResp struct {
		TwoFARequired bool   `json:"two_factor_required"`
		ChallengeID   string `json:"challenge_id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &loginResp)
	if loginResp.ChallengeID == "" {
		t.Fatalf("expected challenge id")
	}

	valid, _ := auth.ComputeTOTPCode(secret, time.Now().UTC(), auth.DefaultTOTPConfig())
	invalid := valid[:5]
	if valid[5] != '0' {
		invalid += "0"
	} else {
		invalid += "1"
	}
	raw2fa, _ := json.Marshal(map[string]any{"challenge_id": loginResp.ChallengeID, "code": invalid})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login/2fa", bytes.NewReader(raw2fa))
	req2.RemoteAddr = "10.0.0.1:1234"
	req2.Header.Set("User-Agent", "test-agent")
	rr2 := httptest.NewRecorder()
	authHandler.Login2FA(rr2, req2)
	if rr2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var username, action, details string
	if err := db.QueryRowContext(ctx, `SELECT username, action, details FROM audit_log WHERE action=? ORDER BY id DESC LIMIT 1`, "auth.2fa.challenge.fail").
		Scan(&username, &action, &details); err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if username != "alice" || action != "auth.2fa.challenge.fail" {
		t.Fatalf("unexpected audit row: username=%q action=%q details=%q", username, action, details)
	}
}
