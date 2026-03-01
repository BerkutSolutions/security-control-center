package handlers

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/bootstrap"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/gui"
)

type AuthHandler struct {
	cfg            *config.AppConfig
	users          store.UsersStore
	sessions       store.SessionStore
	incidents      store.IncidentsStore
	twoFA          store.Auth2FAStore
	passkeys       store.PasskeysStore
	sessionManager *auth.SessionManager
	policy         *rbac.Policy
	audits         store.AuditStore
	logger         *utils.Logger
}

const HealthcheckCookieName = "berkut_healthcheck"

const healthcheckCookiePath = "/healthcheck"
const healthcheckCookieMaxAgeSeconds = 30
const healthcheckCookieMaxSkew = 15 * time.Second

func NewAuthHandler(cfg *config.AppConfig, users store.UsersStore, sessions store.SessionStore, incidents store.IncidentsStore, twoFA store.Auth2FAStore, passkeys store.PasskeysStore, sm *auth.SessionManager, policy *rbac.Policy, audits store.AuditStore, logger *utils.Logger) *AuthHandler {
	return &AuthHandler{cfg: cfg, users: users, sessions: sessions, incidents: incidents, twoFA: twoFA, passkeys: passkeys, sessionManager: sm, policy: policy, audits: audits, logger: logger}
}

func setHealthcheckCookie(w http.ResponseWriter, r *http.Request, cfg *config.AppConfig, enabled bool) {
	cookieSecure := isSecureRequest(r, cfg)
	if !enabled {
		http.SetCookie(w, &http.Cookie{
			Name:     HealthcheckCookieName,
			Value:    "",
			Path:     healthcheckCookiePath,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
		return
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(healthcheckCookieMaxAgeSeconds) * time.Second)
	http.SetCookie(w, &http.Cookie{
		Name:     HealthcheckCookieName,
		Value:    "v1:" + strconv.FormatInt(now.UnixMilli(), 10),
		Path:     healthcheckCookiePath,
		MaxAge:   healthcheckCookieMaxAgeSeconds,
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Safety net: always ensure default admin exists before processing logins.
	if err := bootstrap.EnsureDefaultAdminWithStore(r.Context(), h.users, h.cfg, h.logger); err != nil && h.logger != nil {
		h.logger.Errorf("ensure default admin: %v", err)
	}
	lang := preferredLang(r)
	var cred auth.Credentials
	if err := json.NewDecoder(r.Body).Decode(&cred); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cred.Username = strings.ToLower(strings.TrimSpace(cred.Username))
	if err := utils.ValidateUsername(cred.Username); err != nil {
		http.Error(w, "invalid username", http.StatusBadRequest)
		return
	}
	user, roles, err := h.users.FindByUsername(r.Context(), cred.Username)
	if err != nil || user == nil || !user.Active {
		h.audits.Log(r.Context(), cred.Username, "auth.login_failed", "user missing or inactive")
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	now := time.Now().UTC()
	if isPermanentLock(user) {
		h.audits.Log(r.Context(), cred.Username, "auth.login_blocked", "permanent")
		http.Error(w, localized(lang, "auth.lockedPermanent"), http.StatusForbidden)
		return
	}
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		msg := localizedUntil(lang, "auth.lockedUntil", *user.LockedUntil)
		h.audits.Log(r.Context(), cred.Username, "auth.login_blocked", msg)
		http.Error(w, msg, http.StatusForbidden)
		return
	}
	singleAttempt := user.LockStage >= 1
	if user.LockedUntil != nil && now.After(*user.LockedUntil) {
		user.LockedUntil = nil
		user.FailedAttempts = 0
	}
	ph, _ := auth.ParsePasswordHash(user.PasswordHash, user.Salt)
	ok, err := auth.VerifyPassword(cred.Password, h.cfg.Pepper, ph)
	if err != nil || !ok {
		user.LastFailedAt = &now
		if user.LockStage == 0 && !singleAttempt {
			user.FailedAttempts++
			if user.FailedAttempts >= 5 {
				applyLockout(user, 1, now, "auto")
				h.audits.Log(r.Context(), cred.Username, "auth.lockout", "stage=1 dur=1h")
				h.ensureAuthLockoutIncident(r.Context(), user, 1, now)
				_ = h.users.Update(r.Context(), user, nil)
				msg := localizedUntil(lang, "auth.lockedUntil", *user.LockedUntil)
				http.Error(w, msg, http.StatusForbidden)
				return
			}
			if user.FailedAttempts == 4 {
				_ = h.users.Update(r.Context(), user, nil)
				http.Error(w, localized(lang, "auth.lockoutSoon"), http.StatusUnauthorized)
				return
			}
			_ = h.users.Update(r.Context(), user, nil)
		} else {
			nextStage := user.LockStage + 1
			if nextStage > 6 {
				nextStage = 6
			}
			applyLockout(user, nextStage, now, "auto")
			h.audits.Log(r.Context(), cred.Username, "auth.lockout", "stage="+strconv.Itoa(nextStage))
			h.ensureAuthLockoutIncident(r.Context(), user, nextStage, now)
			_ = h.users.Update(r.Context(), user, nil)
			if isPermanentLock(user) {
				http.Error(w, localized(lang, "auth.lockedPermanent"), http.StatusForbidden)
				return
			}
			msg := localizedUntil(lang, "auth.lockedUntil", *user.LockedUntil)
			http.Error(w, msg, http.StatusForbidden)
			return
		}
		h.audits.Log(r.Context(), cred.Username, "auth.login_failed", "invalid password")
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if user.TOTPEnabled {
		if h.twoFA == nil {
			http.Error(w, localized(lang, "auth.2fa.misconfigured"), http.StatusInternalServerError)
			return
		}
		_ = h.twoFA.DeleteExpiredChallenges(r.Context(), now)
		chID, err := h.twoFA.CreateChallenge(r.Context(), user.ID, clientIP(r, h.cfg), r.UserAgent(), now.Add(5*time.Minute))
		if err != nil {
			if h.logger != nil {
				h.logger.Errorf("auth login 2fa challenge create failed for %s: %v", cred.Username, err)
			}
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"two_factor_required": true,
			"challenge_id":        chID,
			"expires_at":          now.Add(5 * time.Minute).UTC(),
		})
		return
	}
	h.finishLogin(w, r, user, roles, now)
}

func (h *AuthHandler) finishLogin(w http.ResponseWriter, r *http.Request, user *store.User, roles []string, now time.Time) {
	sess, err := h.sessionManager.Create(r.Context(), user, roles, clientIP(r, h.cfg), r.UserAgent())
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("auth login session create failed for %s: %v", user.Username, err)
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	user.LastLoginAt = &now
	user.FailedAttempts = 0
	user.LockedUntil = nil
	user.LockReason = ""
	user.LockStage = 0
	user.LastFailedAt = nil
	_ = h.users.Update(r.Context(), user, nil)
	h.resolveAuthLockoutIncident(r.Context(), user, now)
	h.audits.Log(r.Context(), user.Username, "auth.login_success", "")
	cookieSecure := isSecureRequest(r, h.cfg)
	cookie := http.Cookie{
		Name:     SessionCookieName,
		Value:    sess.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	}
	http.SetCookie(w, &cookie)
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    sess.CSRFToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
	// One-time healthcheck page marker (consumed by GET /healthcheck).
	setHealthcheckCookie(w, r, h.cfg, true)
	groups, _ := h.users.UserGroups(r.Context(), user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": auth.UserDTO{
			ID:                    user.ID,
			Username:              user.Username,
			Roles:                 roles,
			Active:                user.Active,
			PasswordSet:           user.PasswordSet,
			RequirePasswordChange: user.RequirePasswordChange,
			PasswordChangedAt:     user.PasswordChangedAt,
			Permissions:           eff.Permissions,
			MenuPermissions:       eff.MenuPermissions,
		},
		"csrf_token": sess.CSRFToken,
		"session":    sess,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	actor := ""
	if ctxSess := r.Context().Value(auth.SessionContextKey); ctxSess != nil {
		actor = ctxSess.(*store.SessionRecord).Username
	}
	if err == nil && cookie.Value != "" {
		_ = h.sessions.DeleteSession(r.Context(), cookie.Value, actor)
	}
	cookieSecure := isSecureRequest(r, h.cfg)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	h.audits.Log(r.Context(), actor, "auth.logout", "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Ping(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	now := time.Now().UTC()
	// Ensure activity timestamp strictly increases even when requests happen in the same clock tick.
	if !sr.LastSeenAt.IsZero() && !now.After(sr.LastSeenAt) {
		now = sr.LastSeenAt.Add(1 * time.Millisecond)
	}
	_ = h.sessions.UpdateActivity(r.Context(), sr.ID, now, h.cfg.EffectiveSessionTTL())
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "last_seen_at": now})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, roles, err := h.users.FindByUsername(r.Context(), sr.Username)
	if err != nil || user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	groups, _ := h.users.UserGroups(r.Context(), user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": auth.UserDTO{
			ID:                    user.ID,
			Username:              user.Username,
			Roles:                 roles,
			Active:                user.Active,
			PasswordSet:           user.PasswordSet,
			RequirePasswordChange: user.RequirePasswordChange,
			PasswordChangedAt:     user.PasswordChangedAt,
			Permissions:           eff.Permissions,
			MenuPermissions:       eff.MenuPermissions,
		},
		"csrf_token": sr.CSRFToken,
	})
}

func (h *AuthHandler) Menu(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, roles, err := h.users.FindByUsername(r.Context(), sr.Username)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	groups, _ := h.users.UserGroups(r.Context(), user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	menu := buildMenu(eff)
	writeJSON(w, http.StatusOK, map[string]interface{}{"menu": menu})
}

func buildMenu(eff store.EffectiveAccess) []map[string]string {
	entries := []struct {
		Perm rbac.Permission
		Name string
		Path string
	}{
		{Perm: "dashboard.view", Name: "dashboard", Path: "dashboard"},
		{Perm: "tasks.view", Name: "tasks", Path: "tasks"},
		{Perm: "monitoring.view", Name: "monitoring", Path: "monitoring"},
		{Perm: "docs.view", Name: "docs", Path: "docs"},
		{Perm: "docs.approval.view", Name: "approvals", Path: "approvals"},
		{Perm: "incidents.view", Name: "incidents", Path: "incidents"},
		// "Registries" entry: controls tab with internal sub-tabs (overview/controls/checks/etc),
		// plus navigation to Assets/Software/Findings.
		{Perm: "controls.view", Name: "controls", Path: "registry"},
		{Perm: "reports.view", Name: "reports", Path: "reports"},
		{Perm: "backups.read", Name: "backups", Path: "backups"},
		{Perm: "accounts.view", Name: "accounts", Path: "accounts"},
		{Perm: "logs.view", Name: "logs", Path: "logs"},
		{Perm: "app.view", Name: "settings", Path: "settings"},
	}
	var menu []map[string]string
	allowed := map[string]struct{}{}
	for _, p := range eff.Permissions {
		allowed[p] = struct{}{}
	}
	menuAllowed := map[string]struct{}{}
	for _, m := range eff.MenuPermissions {
		menuAllowed[m] = struct{}{}
	}
	for _, e := range entries {
		_, ok := allowed[string(e.Perm)]
		if ok {
			if len(menuAllowed) > 0 {
				if _, ok := menuAllowed[e.Name]; !ok {
					if _, ok2 := menuAllowed[e.Path]; !ok2 {
						continue
					}
				}
			}
			menu = append(menu, map[string]string{"name": e.Name, "path": e.Path})
		}
	}
	return menu
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	var payload struct {
		Current  string `json:"current_password"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	user, _, err := h.users.Get(r.Context(), sr.UserID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := utils.ValidatePassword(payload.Password); err != nil {
		http.Error(w, passwordPolicyMessage(preferredLang(r), err), http.StatusBadRequest)
		return
	}
	if user.PasswordSet {
		phCurrent, _ := auth.ParsePasswordHash(user.PasswordHash, user.Salt)
		ok, _ := auth.VerifyPassword(payload.Current, h.cfg.Pepper, phCurrent)
		if !ok {
			http.Error(w, localized(preferredLang(r), "accounts.currentPasswordInvalid"), http.StatusBadRequest)
			return
		}
	}
	history, _ := h.users.PasswordHistory(r.Context(), sr.UserID, 10)
	if isPasswordReused(payload.Password, h.cfg.Pepper, user, history) {
		h.audits.Log(r.Context(), sr.Username, "auth.password_reuse_denied", "")
		http.Error(w, localized(preferredLang(r), "accounts.passwordReuseDenied"), http.StatusBadRequest)
		return
	}
	ph, err := auth.HashPassword(payload.Password, h.cfg.Pepper)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.users.UpdatePassword(r.Context(), sr.UserID, ph.Hash, ph.Salt, false); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audits.Log(r.Context(), sr.Username, "auth.password_changed", "")
	// Show healthcheck page again after first password change / forced change.
	setHealthcheckCookie(w, r, h.cfg, true)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func ServeStatic(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := gui.StaticFiles.ReadFile("static/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, name, time.Now(), bytes.NewReader(data))
	}
}

func RedirectToApp(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *AuthHandler) HealthcheckPage(w http.ResponseWriter, r *http.Request) {
	// This page is only available right after login (or first password change),
	// controlled by a short-lived, one-time cookie.
	c, err := r.Cookie(HealthcheckCookieName)
	if err != nil || c == nil || strings.TrimSpace(c.Value) == "" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	raw := strings.TrimSpace(c.Value)
	if !strings.HasPrefix(raw, "v1:") {
		setHealthcheckCookie(w, r, h.cfg, false)
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	issuedAtMS, parseErr := strconv.ParseInt(strings.TrimPrefix(raw, "v1:"), 10, 64)
	if parseErr != nil {
		setHealthcheckCookie(w, r, h.cfg, false)
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	issuedAt := time.UnixMilli(issuedAtMS).UTC()
	now := time.Now().UTC()
	diff := now.Sub(issuedAt)
	if issuedAt.IsZero() || diff < (-2*time.Second) || diff > healthcheckCookieMaxSkew {
		// Too old: treat as invalid and clear.
		setHealthcheckCookie(w, r, h.cfg, false)
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	// Consume the marker to prevent opening without re-login.
	setHealthcheckCookie(w, r, h.cfg, false)
	ServeStatic("healthcheck.html")(w, r)
}

func (h *AuthHandler) PasswordChangePage(w http.ResponseWriter, r *http.Request) {
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	user, _, err := h.users.FindByUsername(r.Context(), sr.Username)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if user.PasswordSet && !user.RequirePasswordChange {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	data, err := gui.StaticFiles.ReadFile("static/password.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, "password.html", time.Now(), bytes.NewReader(data))
}

func lockDuration(stage int) time.Duration {
	switch stage {
	case 1:
		return time.Hour
	case 2:
		return 3 * time.Hour
	case 3:
		return 6 * time.Hour
	case 4:
		return 12 * time.Hour
	case 5:
		return 24 * time.Hour
	default:
		return 0
	}
}

func applyLockout(user *store.User, stage int, now time.Time, reason string) {
	user.LockStage = stage
	user.FailedAttempts = 0
	if stage >= 6 {
		user.LockedUntil = nil
		user.LockReason = reason
		return
	}
	dur := lockDuration(stage)
	if dur <= 0 {
		return
	}
	until := now.Add(dur)
	user.LockedUntil = &until
	user.LockReason = reason
}

func isPermanentLock(user *store.User) bool {
	return user != nil && user.LockStage >= 6 && (user.LockedUntil == nil || time.Now().UTC().Before(*user.LockedUntil))
}

func preferredLang(r *http.Request) string {
	al := r.Header.Get("Accept-Language")
	if strings.HasPrefix(strings.ToLower(al), "ru") {
		return "ru"
	}
	return "en"
}

func localized(lang, key string) string {
	ru := map[string]string{
		"auth.lockoutSoon":                 "\u0412\u0430\u0448\u0430 \u0443\u0447\u0435\u0442\u043d\u0430\u044f \u0437\u0430\u043f\u0438\u0441\u044c \u0431\u0443\u0434\u0435\u0442 \u0437\u0430\u0431\u043b\u043e\u043a\u0438\u0440\u043e\u0432\u0430\u043d\u0430 \u043d\u0430 1 \u0447\u0430\u0441.",
		"auth.lockedPermanent":             "\u0410\u043a\u043a\u0430\u0443\u043d\u0442 \u0437\u0430\u0431\u043b\u043e\u043a\u0438\u0440\u043e\u0432\u0430\u043d. \u041e\u0431\u0440\u0430\u0442\u0438\u0442\u0435\u0441\u044c \u043a \u0430\u0434\u043c\u0438\u043d\u0438\u0441\u0442\u0440\u0430\u0442\u043e\u0440\u0443.",
		"accounts.passwordReuseDenied":     "\u041f\u0430\u0440\u043e\u043b\u044c \u0443\u0436\u0435 \u0438\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u043b\u0441\u044f. \u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0434\u0440\u0443\u0433\u043e\u0439.",
		"accounts.currentPasswordInvalid":  "\u0422\u0435\u043a\u0443\u0449\u0438\u0439 \u043f\u0430\u0440\u043e\u043b\u044c \u043d\u0435\u0432\u0435\u0440\u0435\u043d",
		"accounts.roleRequired":            "\u0420\u043e\u043b\u044c \u043e\u0431\u044f\u0437\u0430\u0442\u0435\u043b\u044c\u043d\u0430",
		"accounts.clearanceTooHigh":        "\u041d\u0435\u043b\u044c\u0437\u044f \u0432\u044b\u0434\u0430\u0442\u044c \u0434\u043e\u043f\u0443\u0441\u043a \u0432\u044b\u0448\u0435 \u0441\u0432\u043e\u0435\u0433\u043e",
		"accounts.clearanceTagsNotAllowed": "\u041d\u0435\u043b\u044c\u0437\u044f \u043d\u0430\u0437\u043d\u0430\u0447\u0438\u0442\u044c \u044d\u0442\u0438 \u0442\u0435\u0433\u0438 \u0434\u043e\u043f\u0443\u0441\u043a\u0430",
		"accounts.lastSuperadminProtected": "\u041d\u0435\u043b\u044c\u0437\u044f \u0438\u0437\u043c\u0435\u043d\u0438\u0442\u044c \u043f\u043e\u0441\u043b\u0435\u0434\u043d\u0435\u0433\u043e \u0441\u0443\u043f\u0435\u0440-\u0430\u0434\u043c\u0438\u043d\u0430",
		"accounts.selfLockoutPrevented":    "\u041e\u043f\u0435\u0440\u0430\u0446\u0438\u044f \u0437\u0430\u043f\u0440\u0435\u0449\u0435\u043d\u0430: \u043f\u0440\u0438\u0432\u0435\u043b\u0430 \u0431\u044b \u043a \u043f\u043e\u0442\u0435\u0440\u0435 \u0434\u043e\u0441\u0442\u0443\u043f\u0430",
		"accounts.roleSystemProtected":     "\u0421\u0438\u0441\u0442\u0435\u043c\u043d\u0443\u044e \u0440\u043e\u043b\u044c \u043d\u0435\u043b\u044c\u0437\u044f \u0438\u0437\u043c\u0435\u043d\u0438\u0442\u044c \u0438\u043b\u0438 \u0443\u0434\u0430\u043b\u0438\u0442\u044c",
		"errors.roleTemplateNotFound":      "\u0428\u0430\u0431\u043b\u043e\u043d \u0440\u043e\u043b\u0438 \u043d\u0435 \u043d\u0430\u0439\u0434\u0435\u043d",
	}
	en := map[string]string{
		"auth.lockoutSoon":                 "Your account will be locked for 1 hour.",
		"auth.lockedPermanent":             "Account is locked. Contact administrator.",
		"accounts.passwordReuseDenied":     "Password was used recently. Choose a new one.",
		"accounts.currentPasswordInvalid":  "Current password is invalid",
		"accounts.roleRequired":            "Role is required",
		"accounts.clearanceTooHigh":        "Clearance level exceeds your own",
		"accounts.clearanceTagsNotAllowed": "Clearance tags are not allowed",
		"accounts.lastSuperadminProtected": "Cannot modify the last superadmin",
		"accounts.selfLockoutPrevented":    "Operation blocked to avoid self-lockout",
		"accounts.roleSystemProtected":     "System role cannot be modified or deleted",
		"errors.roleTemplateNotFound":      "Role template not found",
	}
	ru["accounts.groupSystemProtected"] = "\u0421\u0438\u0441\u0442\u0435\u043c\u043d\u0443\u044e \u0433\u0440\u0443\u043f\u043f\u0443 \u043d\u0435\u043b\u044c\u0437\u044f \u0438\u0437\u043c\u0435\u043d\u0438\u0442\u044c \u0438\u043b\u0438 \u0443\u0434\u0430\u043b\u0438\u0442\u044c"
	en["accounts.groupSystemProtected"] = "System group cannot be modified or deleted"
	ru["reports.error.chartNotFound"] = "\u0413\u0440\u0430\u0444\u0438\u043a \u043d\u0435 \u043d\u0430\u0439\u0434\u0435\u043d"
	ru["reports.error.snapshotRequired"] = "\u041d\u0443\u0436\u0435\u043d \u0441\u043d\u0430\u043f\u0448\u043e\u0442"
	ru["reports.error.exportChartsUnavailable"] = "\u0414\u043b\u044f \u044d\u043a\u0441\u043f\u043e\u0440\u0442\u0430 \u0433\u0440\u0430\u0444\u0438\u043a\u043e\u0432 \u043d\u0443\u0436\u0435\u043d \u043b\u043e\u043a\u0430\u043b\u044c\u043d\u044b\u0439 \u043a\u043e\u043d\u0432\u0435\u0440\u0442\u0435\u0440"
	en["reports.error.chartNotFound"] = "Chart not found"
	en["reports.error.snapshotRequired"] = "Snapshot required"
	en["reports.error.exportChartsUnavailable"] = "Chart export requires a local converter"
	ru["docs.onlyoffice.disabled"] = "OnlyOffice \u043e\u0442\u043a\u043b\u044e\u0447\u0435\u043d"
	ru["docs.onlyoffice.unsupportedFormat"] = "OnlyOffice \u043f\u043e\u0434\u0434\u0435\u0440\u0436\u0438\u0432\u0430\u0435\u0442 \u0442\u043e\u043b\u044c\u043a\u043e DOCX"
	ru["docs.onlyoffice.invalidToken"] = "\u041d\u0435\u0434\u0435\u0439\u0441\u0442\u0432\u0438\u0442\u0435\u043b\u044c\u043d\u044b\u0439 \u0442\u043e\u043a\u0435\u043d OnlyOffice"
	ru["docs.onlyoffice.misconfigured"] = "OnlyOffice \u043d\u0430\u0441\u0442\u0440\u043e\u0435\u043d \u043d\u0435\u043a\u043e\u0440\u0440\u0435\u043a\u0442\u043d\u043e"
	ru["docs.onlyoffice.saveReason"] = "\u0420\u0435\u0434\u0430\u043a\u0442\u0438\u0440\u043e\u0432\u0430\u043d\u0438\u0435 \u0432 OnlyOffice"
	ru["docs.onlyoffice.forceSaveFailed"] = "\u041d\u0435 \u0443\u0434\u0430\u043b\u043e\u0441\u044c \u0432\u044b\u043f\u043e\u043b\u043d\u0438\u0442\u044c \u0441\u043e\u0445\u0440\u0430\u043d\u0435\u043d\u0438\u0435 \u0432 OnlyOffice"
	ru["docs.onlyoffice.forceSaveNoVersion"] = "\u0421\u043e\u0445\u0440\u0430\u043d\u0435\u043d\u0438\u0435 \u0437\u0430\u043f\u0440\u043e\u0448\u0435\u043d\u043e, \u043d\u043e \u043d\u043e\u0432\u0430\u044f \u0432\u0435\u0440\u0441\u0438\u044f \u0434\u043e\u043a\u0443\u043c\u0435\u043d\u0442\u0430 \u043d\u0435 \u0431\u044b\u043b\u0430 \u0441\u043e\u0437\u0434\u0430\u043d\u0430"
	en["docs.onlyoffice.disabled"] = "OnlyOffice is disabled"
	en["docs.onlyoffice.unsupportedFormat"] = "Only DOCX is supported for OnlyOffice editing"
	en["docs.onlyoffice.invalidToken"] = "Invalid OnlyOffice token"
	en["docs.onlyoffice.misconfigured"] = "OnlyOffice is misconfigured"
	en["docs.onlyoffice.saveReason"] = "Edited in OnlyOffice"
	en["docs.onlyoffice.forceSaveFailed"] = "OnlyOffice save failed"
	en["docs.onlyoffice.forceSaveNoVersion"] = "Save was requested, but a new document version was not created"
	if lang == "ru" {
		if v, ok := ru[key]; ok {
			return v
		}
	}
	if v, ok := en[key]; ok {
		return v
	}
	return key
}

func localizedUntil(lang, key string, until time.Time) string {
	format := "2006-01-02 15:04"
	if lang == "ru" {
		return "Р С’Р С”Р С”Р В°РЎС“Р Р…РЎвЂљ Р В·Р В°Р В±Р В»Р С•Р С”Р С‘РЎР‚Р С•Р Р†Р В°Р Р… Р Т‘Р С• " + until.Format(format)
	}
	return "Account locked until " + until.Format(format)
}

func clientIP(r *http.Request, cfg *config.AppConfig) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	ip = strings.TrimSpace(ip)
	if cfg == nil || !isTrustedProxy(ip, cfg.Security.TrustedProxies) {
		return ip
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		if candidate := extractClientIPFromXFF(xff, cfg.Security.TrustedProxies); candidate != "" {
			return candidate
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if parsed := net.ParseIP(realIP); parsed != nil {
			return parsed.String()
		}
	}
	return ip
}

func isSecureRequest(r *http.Request, cfg *config.AppConfig) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if cfg == nil {
		return false
	}
	if cfg.TLSEnabled {
		return true
	}
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if remoteIP == "" {
		remoteIP = strings.TrimSpace(r.RemoteAddr)
	}
	remoteIP = strings.TrimSpace(remoteIP)
	if !isTrustedProxy(remoteIP, cfg.Security.TrustedProxies) {
		return false
	}
	xffProto := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("X-Forwarded-Proto"), ",", 2)[0]))
	return xffProto == "https"
}

func extractClientIPFromXFF(xff string, trusted []string) string {
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(parts[i])
		parsed := net.ParseIP(candidate)
		if parsed == nil {
			continue
		}
		val := parsed.String()
		if !isTrustedProxy(val, trusted) {
			return val
		}
	}
	return ""
}

func isTrustedProxy(ip string, trusted []string) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	for _, raw := range trusted {
		val := strings.TrimSpace(raw)
		if val == "" {
			continue
		}
		if strings.Contains(val, "/") {
			if _, block, err := net.ParseCIDR(val); err == nil && block != nil {
				if ones, bits := block.Mask.Size(); bits > 0 {
					is4 := block.IP != nil && block.IP.To4() != nil
					if (is4 && ones <= 16) || (!is4 && ones <= 48) {
						continue
					}
				}
				if block.Contains(parsed) {
					return true
				}
			}
			continue
		}
		if parsed.Equal(net.ParseIP(val)) {
			return true
		}
	}
	return false
}
