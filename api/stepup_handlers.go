package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func (s *Server) StepupStatus(w http.ResponseWriter, r *http.Request) {
	sr, ok := sessionFromRequest(r)
	if !ok || sr == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, user, methods, ok := s.stepupContext(r, sr)
	if !ok {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	required := false
	modelEnabled := s.isBehaviorModelEnabled(r.Context())
	if !modelEnabled {
		payload := s.stepupStatePayload(state, user, methods, false)
		payload["behavior_model_enabled"] = false
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if state != nil {
		required = state.StepupRequired
	}
	payload := s.stepupStatePayload(state, user, methods, required)
	payload["behavior_model_enabled"] = true
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) StepupVerifyPassword(w http.ResponseWriter, r *http.Request) {
	sr, ok := sessionFromRequest(r)
	if !ok || sr == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, user, _, ok := s.stepupContext(r, sr)
	if !ok || user == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !s.isBehaviorModelEnabled(r.Context()) {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, nil, false))
		return
	}
	now := time.Now().UTC()
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		http.Error(w, "auth.stepup.locked", http.StatusLocked)
		return
	}
	var payload struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "common.badRequest", http.StatusBadRequest)
		return
	}
	ph, err := auth.ParsePasswordHash(user.PasswordHash, user.Salt)
	if err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	valid, err := auth.VerifyPassword(strings.TrimSpace(payload.Password), s.cfg.Pepper, ph)
	if err != nil || !valid {
		st, _ := s.registerStepupFailure(r.Context(), sr, "password")
		writeJSON(w, http.StatusUnauthorized, s.stepupStatePayload(st, user, nil, true))
		return
	}
	state.PasswordVerified = true
	if err := s.behaviorRiskStore.SaveState(r.Context(), state); err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	if s.audits != nil {
		_ = s.audits.Log(r.Context(), sr.Username, "security.behavior.stepup_password_ok", "")
	}
	writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, nil, true))
}

func (s *Server) StepupVerifyTOTP(w http.ResponseWriter, r *http.Request) {
	sr, ok := sessionFromRequest(r)
	if !ok || sr == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, user, methods, ok := s.stepupContext(r, sr)
	if !ok || user == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !s.isBehaviorModelEnabled(r.Context()) {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
		return
	}
	now := time.Now().UTC()
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		http.Error(w, "auth.stepup.locked", http.StatusLocked)
		return
	}
	if !state.StepupRequired {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
		return
	}
	if !state.PasswordVerified {
		http.Error(w, "auth.stepup.passwordFirst", http.StatusForbidden)
		return
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "common.badRequest", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(payload.Code)
	if code == "" {
		http.Error(w, "auth.2fa.codeRequired", http.StatusBadRequest)
		return
	}
	okCode, err := s.verifyUserTOTP(user, code, now)
	if err != nil || !okCode {
		st, _ := s.registerStepupFailure(r.Context(), sr, "totp")
		writeJSON(w, http.StatusUnauthorized, s.stepupStatePayload(st, user, methods, true))
		return
	}
	if err := s.completeStepup(r.Context(), sr); err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	state, _ = s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
	writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
}

func (s *Server) StepupPasskeyBegin(w http.ResponseWriter, r *http.Request) {
	sr, ok := sessionFromRequest(r)
	if !ok || sr == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, user, methods, ok := s.stepupContext(r, sr)
	if !ok || user == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !s.isBehaviorModelEnabled(r.Context()) {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
		return
	}
	now := time.Now().UTC()
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		http.Error(w, "auth.stepup.locked", http.StatusLocked)
		return
	}
	if !state.StepupRequired {
		writeJSON(w, http.StatusOK, map[string]any{"required": false})
		return
	}
	if !state.PasswordVerified {
		http.Error(w, "auth.stepup.passwordFirst", http.StatusForbidden)
		return
	}
	pkStore := store.NewPasskeysStore(s.db)
	keys, _ := pkStore.ListUserPasskeys(r.Context(), user.ID)
	if len(keys) == 0 {
		http.Error(w, "auth.stepup.passkeyUnavailable", http.StatusBadRequest)
		return
	}
	web, err := s.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	waUser, err := newStepupWebauthnUser(user, keys)
	if err != nil {
		http.Error(w, "auth.passkeys.failed", http.StatusInternalServerError)
		return
	}
	assertion, session, err := web.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil || session == nil {
		http.Error(w, "auth.passkeys.failed", http.StatusInternalServerError)
		return
	}
	expiresAt := now.Add(5 * time.Minute)
	session.Expires = expiresAt
	uid := user.ID
	chID, err := pkStore.CreateChallenge(r.Context(), "stepup", &uid, session, requestIP(r), r.UserAgent(), expiresAt)
	if err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"challenge_id": chID,
		"options":      assertion,
		"expires_at":   expiresAt,
	})
}

func (s *Server) StepupPasskeyFinish(w http.ResponseWriter, r *http.Request) {
	sr, ok := sessionFromRequest(r)
	if !ok || sr == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	state, user, methods, ok := s.stepupContext(r, sr)
	if !ok || user == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !s.isBehaviorModelEnabled(r.Context()) {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
		return
	}
	now := time.Now().UTC()
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		http.Error(w, "auth.stepup.locked", http.StatusLocked)
		return
	}
	if !state.StepupRequired {
		writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
		return
	}
	if !state.PasswordVerified {
		http.Error(w, "auth.stepup.passwordFirst", http.StatusForbidden)
		return
	}
	var payload struct {
		ChallengeID string          `json:"challenge_id"`
		Credential  json.RawMessage `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "common.badRequest", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.ChallengeID) == "" || len(payload.Credential) == 0 {
		http.Error(w, "common.badRequest", http.StatusBadRequest)
		return
	}

	pkStore := store.NewPasskeysStore(s.db)
	ch, err := pkStore.GetChallenge(r.Context(), payload.ChallengeID)
	if err != nil || ch == nil || ch.UserID == nil || *ch.UserID != user.ID || strings.TrimSpace(ch.Kind) != "stepup" {
		http.Error(w, "auth.passkeys.challengeInvalid", http.StatusUnauthorized)
		return
	}
	if now.After(ch.ExpiresAt) {
		_ = pkStore.DeleteChallenge(r.Context(), payload.ChallengeID)
		http.Error(w, "auth.passkeys.challengeExpired", http.StatusUnauthorized)
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(ch.SessionDataJSON), &session); err != nil {
		http.Error(w, "auth.passkeys.challengeInvalid", http.StatusUnauthorized)
		return
	}
	web, err := s.webAuthnForRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	keys, _ := pkStore.ListUserPasskeys(r.Context(), user.ID)
	waUser, err := newStepupWebauthnUser(user, keys)
	if err != nil {
		http.Error(w, "auth.passkeys.failed", http.StatusUnauthorized)
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(payload.Credential)
	if err != nil {
		http.Error(w, "auth.passkeys.failed", http.StatusBadRequest)
		return
	}
	cred, err := web.ValidateLogin(waUser, session, parsed)
	if err != nil || cred == nil {
		st, _ := s.registerStepupFailure(r.Context(), sr, "passkey")
		writeJSON(w, http.StatusUnauthorized, s.stepupStatePayload(st, user, methods, true))
		return
	}
	credID := base64.RawURLEncoding.EncodeToString(cred.ID)
	if rec, _ := pkStore.GetPasskeyByCredentialID(r.Context(), credID); rec != nil {
		_ = pkStore.UpdatePasskeyUsage(r.Context(), rec.ID, int64(cred.Authenticator.SignCount), now)
	}
	_ = pkStore.DeleteChallenge(r.Context(), payload.ChallengeID)
	if err := s.completeStepup(r.Context(), sr); err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	state, _ = s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
	writeJSON(w, http.StatusOK, s.stepupStatePayload(state, user, methods, false))
}

func (s *Server) stepupContext(r *http.Request, sr *store.SessionRecord) (*store.BehaviorRiskState, *store.User, map[string]bool, bool) {
	if sr == nil {
		return nil, nil, nil, false
	}
	user, _, err := s.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		return nil, nil, nil, false
	}
	if s.behaviorRiskStore == nil || !s.isBehaviorModelEnabled(r.Context()) {
		return &store.BehaviorRiskState{UserID: sr.UserID}, user, map[string]bool{"password": true, "totp": user.TOTPEnabled, "passkey": s.userHasPasskey(r, user.ID)}, true
	}
	state, err := s.behaviorRiskStore.GetState(r.Context(), sr.UserID)
	if err != nil || state == nil {
		state = &store.BehaviorRiskState{UserID: sr.UserID}
	}
	methods := map[string]bool{
		"password": true,
		"totp":     user.TOTPEnabled,
		"passkey":  s.userHasPasskey(r, user.ID),
	}
	return state, user, methods, true
}

func (s *Server) userHasPasskey(r *http.Request, userID int64) bool {
	pkStore := store.NewPasskeysStore(s.db)
	items, err := pkStore.ListUserPasskeys(r.Context(), userID)
	return err == nil && len(items) > 0
}

func (s *Server) stepupStatePayload(state *store.BehaviorRiskState, user *store.User, methods map[string]bool, required bool) map[string]any {
	now := time.Now().UTC()
	resp := map[string]any{
		"required":          required,
		"password_verified": state != nil && state.PasswordVerified,
		"failed_attempts":   0,
		"locked":            false,
		"methods":           methods,
	}
	if user != nil {
		resp["user"] = map[string]any{
			"id":       user.ID,
			"username": user.Username,
		}
	}
	if state == nil {
		return resp
	}
	resp["failed_attempts"] = state.FailedStepups
	if state.LockedUntil != nil && now.Before(*state.LockedUntil) {
		resp["locked"] = true
		resp["locked_until"] = state.LockedUntil.UTC()
		resp["lock_seconds"] = int(state.LockedUntil.UTC().Sub(now).Seconds())
	}
	return resp
}

func (s *Server) verifyUserTOTP(user *store.User, code string, now time.Time) (bool, error) {
	if user == nil || s == nil || s.cfg == nil {
		return false, errors.New("invalid context")
	}
	secret := ""
	if strings.TrimSpace(user.TOTPSecretEnc) != "" {
		plain, err := auth.DecryptTOTPSecret(user.TOTPSecretEnc, s.cfg.Pepper)
		if err != nil {
			return false, err
		}
		secret = plain
	}
	if secret == "" {
		secret = strings.TrimSpace(user.TOTPSecret)
	}
	if secret == "" {
		return false, errors.New("missing totp secret")
	}
	return auth.VerifyTOTP(secret, code, now, auth.DefaultTOTPConfig())
}

func sessionFromRequest(r *http.Request) (*store.SessionRecord, bool) {
	if r == nil {
		return nil, false
	}
	raw := r.Context().Value(auth.SessionContextKey)
	if raw == nil {
		return nil, false
	}
	sr, ok := raw.(*store.SessionRecord)
	return sr, ok
}

type stepupWebauthnUser struct {
	user        *store.User
	credentials []webauthn.Credential
}

func newStepupWebauthnUser(u *store.User, passkeys []store.PasskeyRecord) (*stepupWebauthnUser, error) {
	if u == nil {
		return nil, errors.New("nil user")
	}
	creds := make([]webauthn.Credential, 0, len(passkeys))
	for _, pk := range passkeys {
		rawID, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(pk.CredentialID))
		if err != nil || len(rawID) == 0 {
			continue
		}
		cred := webauthn.Credential{
			ID:              rawID,
			PublicKey:       pk.PublicKey,
			AttestationType: pk.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    pk.AAGUID,
				SignCount: uint32(pk.SignCount),
			},
		}
		creds = append(creds, cred)
	}
	return &stepupWebauthnUser{user: u, credentials: creds}, nil
}

func (u *stepupWebauthnUser) WebAuthnID() []byte {
	if u == nil || u.user == nil {
		return []byte("u:0")
	}
	return []byte("u:" + strconv.FormatInt(u.user.ID, 10))
}

func (u *stepupWebauthnUser) WebAuthnName() string {
	if u == nil || u.user == nil {
		return ""
	}
	return strings.TrimSpace(u.user.Username)
}

func (u *stepupWebauthnUser) WebAuthnDisplayName() string {
	if u == nil || u.user == nil {
		return ""
	}
	name := strings.TrimSpace(u.user.FullName)
	if name != "" {
		return name
	}
	return strings.TrimSpace(u.user.Username)
}

func (u *stepupWebauthnUser) WebAuthnCredentials() []webauthn.Credential {
	if u == nil {
		return nil
	}
	return u.credentials
}

func (u *stepupWebauthnUser) WebAuthnIcon() string { return "" }

func (s *Server) webAuthnForRequest(r *http.Request) (*webauthn.WebAuthn, error) {
	if s == nil || s.cfg == nil {
		return nil, errors.New("auth.passkeys.misconfigured")
	}
	if !s.cfg.Security.WebAuthn.Enabled {
		return nil, errors.New("auth.passkeys.disabled")
	}
	rpID := strings.TrimSpace(s.cfg.Security.WebAuthn.RPID)
	origins := make([]string, 0, len(s.cfg.Security.WebAuthn.Origins))
	for _, origin := range s.cfg.Security.WebAuthn.Origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	homeOrDev := s.cfg.IsHomeMode() || strings.EqualFold(strings.TrimSpace(s.cfg.AppEnv), "dev")
	if rpID == "" {
		if !homeOrDev {
			return nil, errors.New("auth.passkeys.misconfigured")
		}
		rpID = strings.ToLower(strings.TrimSpace(strings.SplitN(strings.TrimSpace(r.Host), ":", 2)[0]))
	}
	if len(origins) == 0 {
		if !homeOrDev {
			return nil, errors.New("auth.passkeys.misconfigured")
		}
		scheme := "http"
		if isHTTPSRequest(r, s.cfg) {
			scheme = "https"
		}
		if strings.TrimSpace(r.Host) != "" {
			origins = []string{scheme + "://" + strings.TrimSpace(r.Host)}
		}
	}
	if rpID == "" || len(origins) == 0 {
		return nil, errors.New("auth.passkeys.misconfigured")
	}
	cfg := &webauthn.Config{
		RPID:          rpID,
		RPDisplayName: strings.TrimSpace(s.cfg.Security.WebAuthn.RPName),
		RPOrigins:     origins,
	}
	return webauthn.New(cfg)
}
