package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (h *AuthHandler) PasskeysList(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil || h.users == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	keys, err := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	type item struct {
		ID         int64      `json:"id"`
		Name       string     `json:"name"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	}
	out := make([]item, 0, len(keys))
	for _, k := range keys {
		out = append(out, item{ID: k.ID, Name: k.Name, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *AuthHandler) PasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
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
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = localized(lang, "auth.passkeys.defaultName")
	}

	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	existing, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
	waUser, _ := newWebAuthnUser(user, existing)
	exclude := webauthn.Credentials(waUser.WebAuthnCredentials()).CredentialDescriptors()

	creation, session, err := web.BeginRegistration(
		waUser,
		webauthn.WithExclusions(exclude),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationPreferred}),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
	)
	if err != nil || session == nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().UTC().Add(8 * time.Minute)
	session.Expires = expiresAt
	uid := user.ID
	chID, err := h.passkeys.CreateChallenge(r.Context(), "register", &uid, session, clientIP(r, h.cfg), r.UserAgent(), expiresAt)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.audits.Log(r.Context(), user.Username, "auth.passkey.register.begin", "")
	writeJSON(w, http.StatusOK, map[string]any{"challenge_id": chID, "name": name, "options": creation, "expires_at": expiresAt})
}

func (h *AuthHandler) PasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
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
		Name        string          `json:"name"`
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

	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	if ch.UserID == nil || *ch.UserID != sr.UserID {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	passkeys, _ := h.passkeys.ListUserPasskeys(r.Context(), user.ID)
	waUser, _ := newWebAuthnUser(user, passkeys)

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(ch.SessionDataJSON), &session); err != nil {
		http.Error(w, localized(lang, "auth.passkeys.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(payload.Credential)
	if err != nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusBadRequest)
		return
	}
	cred, err := web.CreateCredential(waUser, session, parsed)
	if err != nil || cred == nil {
		http.Error(w, localized(lang, "auth.passkeys.failed"), http.StatusBadRequest)
		return
	}

	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	trans, _ := json.Marshal(cred.Transport)
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = localized(lang, "auth.passkeys.defaultName")
	}
	id, err := h.passkeys.CreatePasskey(r.Context(), &store.PasskeyRecord{
		UserID:          user.ID,
		Name:            name,
		CredentialID:    credID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		TransportsJSON:  string(bytes.TrimSpace(trans)),
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       int64(cred.Authenticator.SignCount),
	})
	_ = h.passkeys.DeleteChallenge(r.Context(), chID)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.audits.Log(r.Context(), user.Username, "auth.passkey.create", "id="+strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AuthHandler) PasskeyRename(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	rawID := strings.TrimSpace(pathParams(r)["id"])
	id, _ := strconv.ParseInt(rawID, 10, 64)
	if id <= 0 {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	rec, err := h.passkeys.GetPasskeyByID(r.Context(), id)
	if err != nil || rec == nil || rec.UserID != sr.UserID {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if err := h.passkeys.RenamePasskey(r.Context(), id, name); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.audits.Log(r.Context(), sr.Username, "auth.passkey.rename", "id="+strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AuthHandler) PasskeyDelete(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.passkeys == nil {
		http.Error(w, localized(lang, "auth.passkeys.misconfigured"), http.StatusInternalServerError)
		return
	}
	rawID := strings.TrimSpace(pathParams(r)["id"])
	id, _ := strconv.ParseInt(rawID, 10, 64)
	if id <= 0 {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	rec, err := h.passkeys.GetPasskeyByID(r.Context(), id)
	if err != nil || rec == nil || rec.UserID != sr.UserID {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if err := h.passkeys.DeletePasskey(r.Context(), id); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.audits.Log(r.Context(), sr.Username, "auth.passkey.delete", "id="+strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
