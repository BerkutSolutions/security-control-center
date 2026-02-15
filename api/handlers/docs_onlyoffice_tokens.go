package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type onlyOfficeLinkToken struct {
	Purpose  string `json:"purpose"`
	DocID    int64  `json:"doc_id"`
	Version  int    `json:"version"`
	Username string `json:"username"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

func signOnlyOfficeLinkToken(secret string, claims onlyOfficeLinkToken) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("empty onlyoffice secret")
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmacSHA256([]byte(secret), []byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac)
	return payload + "." + sig, nil
}

func parseOnlyOfficeLinkToken(secret, token string, now time.Time) (*onlyOfficeLinkToken, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("empty onlyoffice secret")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	payloadPart := strings.TrimSpace(parts[0])
	sigPart := strings.TrimSpace(parts[1])
	if payloadPart == "" || sigPart == "" {
		return nil, errors.New("invalid token payload")
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return nil, errors.New("invalid token signature")
	}
	wantSig := hmacSHA256([]byte(secret), []byte(payloadPart))
	if subtle.ConstantTimeCompare(gotSig, wantSig) != 1 {
		return nil, errors.New("invalid token signature")
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return nil, errors.New("invalid token payload")
	}
	var claims onlyOfficeLinkToken
	if err := json.Unmarshal(rawPayload, &claims); err != nil {
		return nil, errors.New("invalid token payload")
	}
	if claims.Exp <= 0 || now.UTC().Unix() >= claims.Exp {
		return nil, errors.New("token expired")
	}
	if claims.DocID <= 0 || claims.Purpose == "" {
		return nil, errors.New("invalid token claims")
	}
	return &claims, nil
}

func buildOnlyOfficeJWT(secret string, claims map[string]any) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("empty onlyoffice secret")
	}
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerRaw, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsRaw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	headerPart := base64.RawURLEncoding.EncodeToString(headerRaw)
	payloadPart := base64.RawURLEncoding.EncodeToString(claimsRaw)
	signingInput := headerPart + "." + payloadPart
	signature := hmacSHA256([]byte(secret), []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func verifyOnlyOfficeJWTFromRequest(r *http.Request, secret, headerName string, now time.Time) error {
	token := strings.TrimSpace(r.Header.Get(headerName))
	if token == "" && !strings.EqualFold(headerName, "Authorization") {
		token = strings.TrimSpace(r.Header.Get("Authorization"))
	}
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return errors.New("missing onlyoffice jwt")
	}
	return verifyOnlyOfficeJWT(secret, token, now)
}

func verifyOnlyOfficeJWT(secret, token string, now time.Time) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("empty onlyoffice secret")
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return errors.New("invalid jwt format")
	}
	headerRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid jwt header")
	}
	var header map[string]any
	if err := json.Unmarshal(headerRaw, &header); err != nil {
		return errors.New("invalid jwt header")
	}
	if alg, _ := header["alg"].(string); !strings.EqualFold(strings.TrimSpace(alg), "HS256") {
		return errors.New("invalid jwt algorithm")
	}
	signingInput := parts[0] + "." + parts[1]
	gotSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return errors.New("invalid jwt signature")
	}
	wantSig := hmacSHA256([]byte(secret), []byte(signingInput))
	if subtle.ConstantTimeCompare(gotSig, wantSig) != 1 {
		return errors.New("invalid jwt signature")
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid jwt payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return errors.New("invalid jwt payload")
	}
	if exp, ok := readJWTUnix(payload["exp"]); ok && now.UTC().Unix() >= exp {
		return errors.New("jwt expired")
	}
	if nbf, ok := readJWTUnix(payload["nbf"]); ok && now.UTC().Unix() < nbf {
		return errors.New("jwt not active")
	}
	if iat, ok := readJWTUnix(payload["iat"]); ok && now.UTC().Unix()+30 < iat {
		return errors.New("jwt issued in future")
	}
	return nil
}

func readJWTUnix(v any) (int64, bool) {
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	case json.Number:
		n, err := x.Int64()
		return n, err == nil
	case string:
		var num json.Number = json.Number(strings.TrimSpace(x))
		n, err := num.Int64()
		return n, err == nil
	default:
		return 0, false
	}
}

func hmacSHA256(secret, payload []byte) []byte {
	m := hmac.New(sha256.New, secret)
	_, _ = m.Write(payload)
	return m.Sum(nil)
}

func buildOnlyOfficeKey(docID int64, ver int) string {
	return fmt.Sprintf("doc-%d-v%d", docID, ver)
}
