package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (h *AuthHandler) Login2FAPasskeyBegin(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil || h.users == nil || h.twoFA == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	web, err := h.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, localized(lang, err.Error()), http.StatusBadRequest)
		return
	}
	var payload struct {
		ChallengeID string `json:"challenge_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	chID := strings.TrimSpace(payload.ChallengeID)
	if chID == "" {
		http.Error(w, localized(lang, "auth.2fa.challengeMissing"), http.StatusBadRequest)
		return
	}
	ch, err := h.twoFA.GetChallenge(r.Context(), chID)
	if err != nil || ch == nil {
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	if now.After(ch.ExpiresAt) {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		http.Error(w, localized(lang, "auth.2fa.challengeExpired"), http.StatusUnauthorized)
		return
	}
	user, _, err := h.users.Get(r.Context(), ch.UserID)
	if err != nil || user == nil || !user.Active {
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	keys, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
	waUser, _ := newWebAuthnUser(user, keys)
	assertion, session, err := web.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil || session == nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusInternalServerError)
		return
	}
	expiresAt := now.Add(5 * time.Minute)
	session.Expires = expiresAt
	uid := user.ID
	wchID, err := h.passkeys.CreateChallenge(r.Context(), "login2fa", &uid, session, clientIP(r, h.cfg), r.UserAgent(), expiresAt)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"webauthn_challenge_id": wchID, "options": assertion, "expires_at": expiresAt})
}

func (h *AuthHandler) Login2FAPasskeyFinish(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil || h.users == nil || h.twoFA == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	web, err := h.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, localized(lang, err.Error()), http.StatusBadRequest)
		return
	}
	var payload struct {
		ChallengeID         string          `json:"challenge_id"`
		WebAuthnChallengeID string          `json:"webauthn_challenge_id"`
		Credential          json.RawMessage `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	chID := strings.TrimSpace(payload.ChallengeID)
	wchID := strings.TrimSpace(payload.WebAuthnChallengeID)
	if chID == "" || wchID == "" || len(payload.Credential) == 0 {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	ch, err := h.twoFA.GetChallenge(r.Context(), chID)
	if err != nil || ch == nil {
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	if now.After(ch.ExpiresAt) {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		http.Error(w, localized(lang, "auth.2fa.challengeExpired"), http.StatusUnauthorized)
		return
	}
	wch, err := h.passkeys.GetChallenge(r.Context(), wchID)
	if err != nil || wch == nil {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	if now.After(wch.ExpiresAt) {
		_ = h.passkeys.DeleteChallenge(r.Context(), wchID)
		http.Error(w, localized(lang, "auth.passkeys.challengeExpired"), http.StatusUnauthorized)
		return
	}
	if wch.UserID == nil || *wch.UserID != ch.UserID {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	user, roles, err := h.users.Get(r.Context(), ch.UserID)
	if err != nil || user == nil || !user.Active {
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	keys, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
	waUser, _ := newWebAuthnUser(user, keys)
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(wch.SessionDataJSON), &session); err != nil {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(payload.Credential)
	if err != nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusBadRequest)
		return
	}
	cred, err := web.ValidateLogin(waUser, session, parsed)
	if err != nil || cred == nil {
		_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.challenge.fail", "passkey_invalid")
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusUnauthorized)
		return
	}
	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	if pk, err := h.passkeys.GetPasskeyByCredentialID(r.Context(), credID); err == nil && pk != nil {
		_ = h.passkeys.UpdatePasskeyUsage(r.Context(), pk.ID, int64(cred.Authenticator.SignCount), now)
	}
	_ = h.passkeys.DeleteChallenge(r.Context(), wchID)
	_ = h.twoFA.DeleteChallenge(r.Context(), chID)
	_ = h.audits.Log(r.Context(), user.Username, "auth.passkey.2fa.used", "")
	h.finishLogin(w, r, user, roles, now)
}
