package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (h *AuthHandler) PasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil || h.users == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	web, err := h.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, localized(lang, err.Error()), http.StatusBadRequest)
		return
	}
	var payload struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	username := strings.ToLower(strings.TrimSpace(payload.Username))

	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	if username != "" {
		user, _, err := h.users.FindByUsername(r.Context(), username)
		if err != nil || user == nil || !user.Active {
			http.Error(w, localized(lang, "auth.invalidCredentials"), http.StatusUnauthorized)
			return
		}
		keys, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
		waUser, _ := newWebAuthnUser(user, keys)
		assertion, session, err := web.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationPreferred))
		if err != nil || session == nil {
			http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusInternalServerError)
			return
		}
		session.Expires = expiresAt
		uid := user.ID
		chID, err := h.passkeys.CreateChallenge(r.Context(), "login", &uid, session, clientIP(r, h.cfg), r.UserAgent(), expiresAt)
		if err != nil {
			http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"challenge_id": chID, "options": assertion, "expires_at": expiresAt})
		return
	}

	assertion, session, err := web.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil || session == nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusInternalServerError)
		return
	}
	session.Expires = expiresAt
	chID, err := h.passkeys.CreateChallenge(r.Context(), "discoverable", nil, session, clientIP(r, h.cfg), r.UserAgent(), expiresAt)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"challenge_id": chID, "options": assertion, "expires_at": expiresAt})
}

func (h *AuthHandler) PasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil || h.users == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	web, err := h.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, localized(lang, err.Error()), http.StatusBadRequest)
		return
	}
	var payload struct {
		ChallengeID string          `json:"challenge_id"`
		Credential  json.RawMessage `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	chID := strings.TrimSpace(payload.ChallengeID)
	if chID == "" || len(payload.Credential) == 0 {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	ch, err := h.passkeys.GetChallenge(r.Context(), chID)
	if err != nil || ch == nil {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	if now.After(ch.ExpiresAt) {
		_ = h.passkeys.DeleteChallenge(r.Context(), chID)
		http.Error(w, localized(lang, "auth.passkeys.challengeExpired"), http.StatusUnauthorized)
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(ch.SessionDataJSON), &session); err != nil {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}

	var user *store.User
	var roles []string
	var pk *store.PasskeyRecord
	var signCount int64 = -1
	if ch.UserID != nil && *ch.UserID > 0 {
		user, roles, _ = h.users.Get(r.Context(), *ch.UserID)
	} else {
		handler := func(rawID, userHandle []byte) (webauthn.User, error) {
			_ = userHandle
			credID := base64.RawURLEncoding.EncodeToString(rawID)
			found, err := h.passkeys.GetPasskeyByCredentialID(r.Context(), credID)
			if err != nil || found == nil {
				return nil, errors.New("not found")
			}
			u, _, err := h.users.Get(r.Context(), found.UserID)
			if err != nil || u == nil {
				return nil, errors.New("not found")
			}
			all, _ := h.passkeys.ListUserPasskeys(r.Context(), u.ID)
			return newWebAuthnUser(u, all)
		}
		waUser, cred, err := web.FinishPasskeyLogin(handler, session, requestWithBody(r, payload.Credential))
		if err != nil || waUser == nil || cred == nil {
			http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(string(waUser.WebAuthnID()), "u:")
		uid, _ := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
		if uid > 0 {
			user, roles, _ = h.users.Get(r.Context(), uid)
		}
		credID := base64.RawURLEncoding.EncodeToString(cred.ID)
		pk, _ = h.passkeys.GetPasskeyByCredentialID(r.Context(), credID)
		signCount = int64(cred.Authenticator.SignCount)
	}

	if user == nil || !user.Active {
		http.Error(w, localized(lang, "auth.invalidCredentials"), http.StatusUnauthorized)
		return
	}

	if ch.UserID != nil && *ch.UserID > 0 {
		parsed, err := protocol.ParseCredentialRequestResponseBytes(payload.Credential)
		if err != nil {
			http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusBadRequest)
			return
		}
		keys, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
		waUser, _ := newWebAuthnUser(user, keys)
		cred, err := web.ValidateLogin(waUser, session, parsed)
		if err != nil || cred == nil {
			_ = h.audits.Log(r.Context(), user.Username, "auth.passkey.login_failed", "")
			http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusUnauthorized)
			return
		}
		credID := base64.RawURLEncoding.EncodeToString(cred.ID)
		pk, _ = h.passkeys.GetPasskeyByCredentialID(r.Context(), credID)
		signCount = int64(cred.Authenticator.SignCount)
	}

	if pk != nil && signCount >= 0 {
		_ = h.passkeys.UpdatePasskeyUsage(r.Context(), pk.ID, signCount, now)
	}
	_ = h.passkeys.DeleteChallenge(r.Context(), chID)
	_ = h.audits.Log(r.Context(), user.Username, "auth.passkey.login_success", "")
	h.finishLogin(w, r, user, roles, now)
}
