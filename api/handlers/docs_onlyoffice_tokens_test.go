package handlers

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestOnlyOfficeLinkTokenRoundtrip(t *testing.T) {
	now := time.Now().UTC()
	token, err := signOnlyOfficeLinkToken("secret", onlyOfficeLinkToken{
		Purpose:  "file",
		DocID:    42,
		Version:  3,
		Username: "admin",
		Iat:      now.Unix(),
		Exp:      now.Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	claims, err := parseOnlyOfficeLinkToken("secret", token, now)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.DocID != 42 || claims.Version != 3 || claims.Purpose != "file" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestOnlyOfficeJWTVerify(t *testing.T) {
	now := time.Now().UTC()
	jwt, err := buildOnlyOfficeJWT("secret", map[string]any{
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("build jwt: %v", err)
	}
	req := httptest.NewRequest("POST", "/cb", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)
	if err := verifyOnlyOfficeJWTFromRequest(req, "secret", "Authorization", now); err != nil {
		t.Fatalf("verify jwt: %v", err)
	}
}
