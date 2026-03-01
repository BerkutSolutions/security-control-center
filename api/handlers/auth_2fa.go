package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"

	"github.com/skip2/go-qrcode"
)

func (h *AuthHandler) Login2FA(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	if h == nil || h.cfg == nil || h.users == nil || h.twoFA == nil {
		http.Error(w, localized(lang, "auth.2fa.misconfigured"), http.StatusInternalServerError)
		return
	}
	var payload struct {
		ChallengeID  string `json:"challenge_id"`
		Code         string `json:"code"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "bad_request")
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	chID := strings.TrimSpace(payload.ChallengeID)
	if chID == "" {
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "challenge_missing")
		http.Error(w, localized(lang, "auth.2fa.challengeMissing"), http.StatusBadRequest)
		return
	}
	ch, err := h.twoFA.GetChallenge(r.Context(), chID)
	if err != nil || ch == nil {
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "challenge_invalid")
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	if now.After(ch.ExpiresAt) {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "challenge_expired")
		http.Error(w, localized(lang, "auth.2fa.challengeExpired"), http.StatusUnauthorized)
		return
	}
	if expected := strings.TrimSpace(ch.IP); expected != "" && expected != strings.TrimSpace(clientIP(r, h.cfg)) {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "challenge_ip_mismatch")
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	if expected := strings.TrimSpace(ch.UserAgent); expected != "" && expected != strings.TrimSpace(r.UserAgent()) {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "challenge_ua_mismatch")
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}
	user, roles, err := h.users.Get(r.Context(), ch.UserID)
	if err != nil || user == nil || !user.Active || !user.TOTPEnabled {
		_ = h.twoFA.DeleteChallenge(r.Context(), chID)
		_ = h.audits.Log(r.Context(), "", "auth.2fa.challenge.fail", "user_invalid_or_inactive")
		http.Error(w, localized(lang, "auth.2fa.challengeInvalid"), http.StatusUnauthorized)
		return
	}

	code := strings.TrimSpace(payload.Code)
	recovery := strings.TrimSpace(payload.RecoveryCode)
	if code == "" && recovery == "" {
		_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.challenge.fail", "code_required")
		http.Error(w, localized(lang, "auth.2fa.codeRequired"), http.StatusBadRequest)
		return
	}

	ok := false
	usedRecovery := false
	if recovery != "" {
		ok, err = h.verifyAndConsumeRecoveryCode(r.Context(), user.ID, recovery, now, clientIP(r, h.cfg), r.UserAgent())
		if ok {
			usedRecovery = true
			_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.recovery.used", "")
		}
	} else {
		ok, err = h.verifyTOTPForUser(user, code, now)
	}
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("auth 2fa verify failed for user %d: %v", user.ID, err)
		}
		_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.challenge.fail", "verify_error")
		if recovery != "" {
			http.Error(w, localized(lang, "auth.2fa.invalidRecovery"), http.StatusUnauthorized)
			return
		}
		http.Error(w, localized(lang, "auth.2fa.invalidCode"), http.StatusUnauthorized)
		return
	}
	if !ok {
		if recovery != "" {
			_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.challenge.fail", "invalid_recovery")
			http.Error(w, localized(lang, "auth.2fa.invalidRecovery"), http.StatusUnauthorized)
			return
		}
		_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.challenge.fail", "invalid_code")
		http.Error(w, localized(lang, "auth.2fa.invalidCode"), http.StatusUnauthorized)
		return
	}
	if !usedRecovery {
		_ = h.migrateLegacyTOTPSecretIfNeeded(r.Context(), user)
	}
	_ = h.twoFA.DeleteChallenge(r.Context(), chID)
	h.finishLogin(w, r, user, roles, now)
}

func (h *AuthHandler) TwoFAStatus(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(preferredLang(r), "common.notFound"), http.StatusNotFound)
		return
	}
	remaining := 0
	if h.twoFA != nil && user.TOTPEnabled {
		if n, err := h.twoFA.CountUnusedRecoveryCodes(r.Context(), user.ID); err == nil {
			remaining = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":                  user.TOTPEnabled,
		"recovery_codes_remaining": remaining,
	})
}

func (h *AuthHandler) TwoFASetup(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if user.TOTPEnabled {
		http.Error(w, localized(lang, "auth.2fa.alreadyEnabled"), http.StatusBadRequest)
		return
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	secretEnc, err := auth.EncryptTOTPSecret(secret, h.cfg.Pepper)
	if err != nil {
		http.Error(w, localized(lang, "auth.2fa.misconfigured"), http.StatusInternalServerError)
		return
	}
	expires := time.Now().UTC().Add(10 * time.Minute)
	if err := h.twoFA.UpsertTOTPSetup(r.Context(), user.ID, secretEnc, expires); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	uri := auth.BuildTOTPProvisioningURI("Berkut SCC", user.Username, secret)
	png, err := qrcode.Encode(uri, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"otpauth_uri":   uri,
		"qr_png_base64": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		"manual_secret": secret,
		"expires_at":    expires,
	})
}

func (h *AuthHandler) TwoFAEnable(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if user.TOTPEnabled {
		http.Error(w, localized(lang, "auth.2fa.alreadyEnabled"), http.StatusBadRequest)
		return
	}
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	setup, err := h.twoFA.GetTOTPSetup(r.Context(), user.ID)
	if err != nil || setup == nil {
		http.Error(w, localized(lang, "auth.2fa.setupMissing"), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	if now.After(setup.ExpiresAt) {
		_ = h.twoFA.DeleteTOTPSetup(r.Context(), user.ID)
		http.Error(w, localized(lang, "auth.2fa.setupExpired"), http.StatusBadRequest)
		return
	}
	secret, err := auth.DecryptTOTPSecret(setup.SecretEnc, h.cfg.Pepper)
	if err != nil || secret == "" {
		http.Error(w, localized(lang, "auth.2fa.misconfigured"), http.StatusInternalServerError)
		return
	}
	ok, err := auth.VerifyTOTP(secret, payload.Code, now, auth.DefaultTOTPConfig())
	if err != nil || !ok {
		http.Error(w, localized(lang, "auth.2fa.invalidCode"), http.StatusUnauthorized)
		return
	}
	secretEnc, err := auth.EncryptTOTPSecret(secret, h.cfg.Pepper)
	if err != nil {
		http.Error(w, localized(lang, "auth.2fa.misconfigured"), http.StatusInternalServerError)
		return
	}
	if err := h.users.SetTOTP(r.Context(), user.ID, true, secretEnc); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.twoFA.DeleteTOTPSetup(r.Context(), user.ID)

	codes, err := auth.GenerateRecoveryCodes(10)
	if err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	hashes := make([]store.RecoveryCodeHash, 0, len(codes))
	for _, c := range codes {
		ph, err := auth.HashRecoveryCode(c, h.cfg.Pepper)
		if err != nil {
			http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
			return
		}
		hashes = append(hashes, store.RecoveryCodeHash{Hash: ph.Hash, Salt: ph.Salt})
	}
	_ = h.twoFA.DeleteRecoveryCodes(r.Context(), user.ID)
	if err := h.twoFA.InsertRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.enable", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "recovery_codes": codes})
}

func (h *AuthHandler) TwoFADisable(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if !user.TOTPEnabled {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	var payload struct {
		Password     string `json:"password"`
		RecoveryCode string `json:"recovery_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Password) == "" || strings.TrimSpace(payload.RecoveryCode) == "" {
		http.Error(w, localized(lang, "auth.2fa.disableRequiresRecovery"), http.StatusBadRequest)
		return
	}
	ph, _ := auth.ParsePasswordHash(user.PasswordHash, user.Salt)
	passOK, _ := auth.VerifyPassword(payload.Password, h.cfg.Pepper, ph)
	if !passOK {
		http.Error(w, localized(lang, "auth.invalidCredentials"), http.StatusUnauthorized)
		return
	}
	now := time.Now().UTC()
	ok, err := h.verifyAndConsumeRecoveryCode(r.Context(), user.ID, payload.RecoveryCode, now, clientIP(r, h.cfg), r.UserAgent())
	if err != nil || !ok {
		http.Error(w, localized(lang, "auth.2fa.invalidRecovery"), http.StatusUnauthorized)
		return
	}
	_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.recovery.used", "")
	if err := h.users.ClearTOTP(r.Context(), user.ID); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	_ = h.twoFA.DeleteRecoveryCodes(r.Context(), user.ID)
	_ = h.twoFA.DeleteChallengesForUser(r.Context(), user.ID)
	_ = h.twoFA.DeleteTOTPSetup(r.Context(), user.ID)
	_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.disable", "self")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AuthHandler) verifyTOTPForUser(user *store.User, code string, now time.Time) (bool, error) {
	if h == nil || user == nil || h.cfg == nil {
		return false, errors.New("invalid context")
	}
	secret := ""
	if strings.TrimSpace(user.TOTPSecretEnc) != "" {
		plain, err := auth.DecryptTOTPSecret(user.TOTPSecretEnc, h.cfg.Pepper)
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

func (h *AuthHandler) migrateLegacyTOTPSecretIfNeeded(ctx context.Context, user *store.User) error {
	if h == nil || h.cfg == nil || user == nil || h.users == nil {
		return nil
	}
	if strings.TrimSpace(user.TOTPSecretEnc) != "" || strings.TrimSpace(user.TOTPSecret) == "" {
		return nil
	}
	secretEnc, err := auth.EncryptTOTPSecret(strings.TrimSpace(user.TOTPSecret), h.cfg.Pepper)
	if err != nil {
		return err
	}
	if err := h.users.SetTOTP(ctx, user.ID, true, secretEnc); err != nil {
		return err
	}
	user.TOTPSecretEnc = secretEnc
	user.TOTPSecret = ""
	return nil
}

func (h *AuthHandler) verifyAndConsumeRecoveryCode(ctx context.Context, userID int64, code string, now time.Time, ip, userAgent string) (bool, error) {
	if h == nil || h.twoFA == nil || h.cfg == nil {
		return false, errors.New("invalid context")
	}
	items, err := h.twoFA.ListUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	code = auth.NormalizeRecoveryCode(code)
	if code == "" {
		return false, auth.ErrInvalidRecoveryCode
	}
	for _, it := range items {
		stored, err := auth.ParsePasswordHash(it.Hash, it.Salt)
		if err != nil {
			continue
		}
		ok, _ := auth.VerifyRecoveryCode(code, h.cfg.Pepper, stored)
		if ok {
			_ = h.twoFA.MarkRecoveryCodeUsed(ctx, it.ID, now, ip, userAgent)
			return true, nil
		}
	}
	return false, nil
}
