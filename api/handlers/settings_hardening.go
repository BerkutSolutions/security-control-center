package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/security/behavior"
	"berkut-scc/core/store"
)

type HardeningHandler struct {
	cfg      *config.AppConfig
	modules  store.AppModuleStateStore
	https    store.AppHTTPSStore
	runtime  store.AppRuntimeStore
	behavior store.BehaviorRiskStore
	users    store.UsersStore
	audits   store.AuditStore
}

type hardeningCheck struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Score          int    `json:"score"`
	MaxScore       int    `json:"max_score"`
	TitleI18NKey   string `json:"title_i18n_key"`
	MessageI18NKey string `json:"message_i18n_key"`
	HintI18NKey    string `json:"hint_i18n_key"`
}

const hardeningSettingsModuleID = "settings.hardening"

type hardeningDLPSettings struct {
	ProtectClipboardAndPrint   bool     `json:"protect_clipboard_and_print"`
	BlockScreenshots           bool     `json:"block_screenshots"`
	AdminExportWithoutApproval bool     `json:"admin_export_without_approval"`
	ApplyMode                  string   `json:"apply_mode"`
	Scope                      []string `json:"scope"`
}

func NewHardeningHandler(cfg *config.AppConfig, modules store.AppModuleStateStore, https store.AppHTTPSStore, runtime store.AppRuntimeStore, behavior store.BehaviorRiskStore, users store.UsersStore, audits store.AuditStore) *HardeningHandler {
	return &HardeningHandler{cfg: cfg, modules: modules, https: https, runtime: runtime, behavior: behavior, users: users, audits: audits}
}

func (h *HardeningHandler) GetBaseline(w http.ResponseWriter, r *http.Request) {
	dlp := loadHardeningDLPSettings(r.Context(), h.cfg, h.modules)
	report := map[string]any{
		"generated_at": time.Now().UTC(),
		"score":        0,
		"max_score":    0,
		"status":       "warning",
		"checks":       []hardeningCheck{},
		"dlp": map[string]any{
			"protect_clipboard_and_print":   dlp.ProtectClipboardAndPrint,
			"block_screenshots":             dlp.BlockScreenshots,
			"admin_export_without_approval": dlp.AdminExportWithoutApproval,
			"apply_mode":                    dlp.ApplyMode,
			"scope":                         dlp.Scope,
		},
	}
	checks := make([]hardeningCheck, 0, 6)
	checks = append(checks, h.checkTLSMode(r))
	checks = append(checks, h.checkTrustedProxies(r))
	checks = append(checks, h.checkSecretsHealth())
	checks = append(checks, h.checkSessionPolicy())
	checks = append(checks, h.checkPasswordPolicy())
	checks = append(checks, h.checkBehaviorModel(r))
	score := 0
	maxScore := 0
	for i := range checks {
		score += checks[i].Score
		maxScore += checks[i].MaxScore
	}
	status := "warning"
	if maxScore > 0 {
		pct := int(float64(score) * 100 / float64(maxScore))
		if pct >= 85 {
			status = "ok"
		} else if pct < 60 {
			status = "failed"
		}
	}
	report["score"] = score
	report["max_score"] = maxScore
	report["status"] = status
	report["checks"] = checks
	_ = h.audits.Log(r.Context(), currentUsername(r), "settings.hardening.read", "score="+strconv.Itoa(score)+"/"+strconv.Itoa(maxScore))
	writeJSON(w, http.StatusOK, report)
}

func (h *HardeningHandler) UpdateBaseline(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	current := loadHardeningDLPSettings(r.Context(), h.cfg, h.modules)
	var payload struct {
		DLP struct {
			ProtectClipboardAndPrint   *bool    `json:"protect_clipboard_and_print"`
			BlockScreenshots           *bool    `json:"block_screenshots"`
			AdminExportWithoutApproval *bool    `json:"admin_export_without_approval"`
			ApplyMode                  string   `json:"apply_mode"`
			Scope                      []string `json:"scope"`
		} `json:"dlp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	next := current
	if payload.DLP.ProtectClipboardAndPrint != nil {
		next.ProtectClipboardAndPrint = *payload.DLP.ProtectClipboardAndPrint
	}
	if payload.DLP.BlockScreenshots != nil {
		next.BlockScreenshots = *payload.DLP.BlockScreenshots
	}
	if payload.DLP.AdminExportWithoutApproval != nil {
		next.AdminExportWithoutApproval = *payload.DLP.AdminExportWithoutApproval
	}
	if payload.DLP.ApplyMode != "" {
		next.ApplyMode = sanitizeDLPApplyMode(payload.DLP.ApplyMode)
	}
	if payload.DLP.Scope != nil {
		next.Scope = sanitizeDLPScope(payload.DLP.Scope)
	}
	raw, err := json.Marshal(map[string]any{
		"dlp": map[string]any{
			"protect_clipboard_and_print":   next.ProtectClipboardAndPrint,
			"block_screenshots":             next.BlockScreenshots,
			"admin_export_without_approval": next.AdminExportWithoutApproval,
			"apply_mode":                    next.ApplyMode,
			"scope":                         next.Scope,
		},
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               hardeningSettingsModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	}); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		_ = h.audits.Log(
			r.Context(),
			currentUsername(r),
			"settings.hardening.dlp.update",
			"protect_clipboard_and_print="+strconv.FormatBool(next.ProtectClipboardAndPrint)+
				"|block_screenshots="+strconv.FormatBool(next.BlockScreenshots)+
				"|admin_export_without_approval="+strconv.FormatBool(next.AdminExportWithoutApproval)+
				"|apply_mode="+next.ApplyMode+
				"|scope="+strings.Join(next.Scope, ","),
		)
	}
	h.GetBaseline(w, r)
}

func loadHardeningDLPSettings(ctx context.Context, cfg *config.AppConfig, modules store.AppModuleStateStore) hardeningDLPSettings {
	out := hardeningDLPSettings{
		ProtectClipboardAndPrint:   true,
		BlockScreenshots:           true,
		AdminExportWithoutApproval: true,
		ApplyMode:                  "protected_only",
		Scope:                      []string{"docs", "reports"},
	}
	if cfg != nil {
		out.ProtectClipboardAndPrint = cfg.Docs.DLP.ProtectClipboardAndPrint
	}
	if modules == nil {
		return out
	}
	st, err := modules.Get(ctx, hardeningSettingsModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return out
	}
	var payload struct {
		DLP struct {
			ProtectClipboardAndPrint   *bool    `json:"protect_clipboard_and_print"`
			BlockScreenshots           *bool    `json:"block_screenshots"`
			AdminExportWithoutApproval *bool    `json:"admin_export_without_approval"`
			ApplyMode                  string   `json:"apply_mode"`
			Scope                      []string `json:"scope"`
		} `json:"dlp"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		return out
	}
	if payload.DLP.ProtectClipboardAndPrint != nil {
		out.ProtectClipboardAndPrint = *payload.DLP.ProtectClipboardAndPrint
	}
	if payload.DLP.BlockScreenshots != nil {
		out.BlockScreenshots = *payload.DLP.BlockScreenshots
	}
	if payload.DLP.AdminExportWithoutApproval != nil {
		out.AdminExportWithoutApproval = *payload.DLP.AdminExportWithoutApproval
	}
	out.ApplyMode = sanitizeDLPApplyMode(payload.DLP.ApplyMode)
	out.Scope = sanitizeDLPScope(payload.DLP.Scope)
	return out
}

func sanitizeDLPApplyMode(raw string) string {
	val := strings.ToLower(strings.TrimSpace(raw))
	switch val {
	case "all":
		return "all"
	default:
		return "protected_only"
	}
}

func sanitizeDLPScope(raw []string) []string {
	if len(raw) == 0 {
		return []string{"docs", "reports"}
	}
	allow := map[string]struct{}{
		"docs":    {},
		"reports": {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	for _, item := range raw {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, ok := allow[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return []string{"docs", "reports"}
	}
	sort.Strings(out)
	return out
}

func (h *HardeningHandler) checkTLSMode(r *http.Request) hardeningCheck {
	check := hardeningCheck{
		ID:             "tls_mode",
		MaxScore:       20,
		TitleI18NKey:   "settings.hardening.check.tlsMode.title",
		HintI18NKey:    "settings.hardening.check.tlsMode.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.tlsMode.warn",
	}
	mode := h.httpsMode(r)
	switch mode {
	case HTTPSModeBuiltinTLS, HTTPSModeExternalProxy:
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.tlsMode.ok"
	case HTTPSModeDisabled:
		if h.deploymentMode(r) == "home" {
			check.Status = "warning"
			check.Score = check.MaxScore / 2
			check.MessageI18NKey = "settings.hardening.check.tlsMode.homeWarn"
		} else {
			check.Status = "failed"
			check.Score = 0
			check.MessageI18NKey = "settings.hardening.check.tlsMode.fail"
		}
	}
	return check
}

func (h *HardeningHandler) checkTrustedProxies(r *http.Request) hardeningCheck {
	check := hardeningCheck{
		ID:             "trusted_proxies",
		MaxScore:       20,
		TitleI18NKey:   "settings.hardening.check.trustedProxies.title",
		HintI18NKey:    "settings.hardening.check.trustedProxies.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.trustedProxies.warn",
	}
	mode := h.httpsMode(r)
	proxies := h.trustedProxies(r)
	if mode != HTTPSModeExternalProxy && mode != HTTPSModeBuiltinTLS {
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.trustedProxies.na"
		return check
	}
	if len(proxies) == 0 {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.trustedProxies.failEmpty"
		return check
	}
	broad := false
	for _, proxy := range proxies {
		if isBroadCIDR(proxy) {
			broad = true
			break
		}
	}
	if broad {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.trustedProxies.failBroad"
		return check
	}
	check.Status = "ok"
	check.Score = check.MaxScore
	check.MessageI18NKey = "settings.hardening.check.trustedProxies.ok"
	return check
}

func (h *HardeningHandler) checkSecretsHealth() hardeningCheck {
	check := hardeningCheck{
		ID:             "secrets_health",
		MaxScore:       30,
		TitleI18NKey:   "settings.hardening.check.secrets.title",
		HintI18NKey:    "settings.hardening.check.secrets.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.secrets.warn",
	}
	secrets := []string{}
	if h.cfg != nil {
		secrets = append(secrets, h.cfg.CSRFKey, h.cfg.Pepper, h.cfg.Docs.EncryptionKey, h.cfg.Backups.EncryptionKey)
	}
	weak := 0
	for _, secret := range secrets {
		if isWeakSecret(secret) {
			weak++
		}
	}
	if len(secrets) == 0 {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.secrets.fail"
		return check
	}
	switch {
	case weak == 0:
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.secrets.ok"
	case weak == len(secrets):
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.secrets.fail"
	default:
		check.Status = "warning"
		check.Score = check.MaxScore / 2
		check.MessageI18NKey = "settings.hardening.check.secrets.warn"
	}
	return check
}

func (h *HardeningHandler) checkSessionPolicy() hardeningCheck {
	check := hardeningCheck{
		ID:             "session_policy",
		MaxScore:       15,
		TitleI18NKey:   "settings.hardening.check.session.title",
		HintI18NKey:    "settings.hardening.check.session.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.session.warn",
	}
	ttl := 24 * time.Hour
	if h.cfg != nil && h.cfg.SessionTTL > 0 {
		ttl = h.cfg.SessionTTL
	}
	if ttl <= 24*time.Hour && ttl >= 15*time.Minute {
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.session.ok"
		return check
	}
	if ttl > 24*time.Hour {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.session.fail"
		return check
	}
	check.Score = check.MaxScore / 2
	return check
}

func (h *HardeningHandler) checkPasswordPolicy() hardeningCheck {
	check := hardeningCheck{
		ID:             "password_policy",
		MaxScore:       15,
		TitleI18NKey:   "settings.hardening.check.password.title",
		HintI18NKey:    "settings.hardening.check.password.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.password.warn",
	}
	if h.cfg == nil {
		check.Status = "failed"
		check.MessageI18NKey = "settings.hardening.check.password.fail"
		return check
	}
	if strings.TrimSpace(h.cfg.Pepper) == "" {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.password.fail"
		return check
	}
	if h.cfg.Security.AuthLockoutIncident {
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.password.ok"
		return check
	}
	check.Status = "warning"
	check.Score = check.MaxScore / 2
	return check
}

func (h *HardeningHandler) checkBehaviorModel(r *http.Request) hardeningCheck {
	check := hardeningCheck{
		ID:             "behavior_model",
		MaxScore:       10,
		TitleI18NKey:   "settings.hardening.check.behaviorModel.title",
		HintI18NKey:    "settings.hardening.check.behaviorModel.hint",
		Status:         "warning",
		MessageI18NKey: "settings.hardening.check.behaviorModel.warn",
	}
	if h.runtime == nil {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.behaviorModel.fail"
		return check
	}
	settings, err := h.runtime.GetRuntimeSettings(r.Context())
	if err != nil || settings == nil {
		check.Status = "failed"
		check.Score = 0
		check.MessageI18NKey = "settings.hardening.check.behaviorModel.fail"
		return check
	}
	if settings.BehaviorModelEnabled {
		check.Status = "ok"
		check.Score = check.MaxScore
		check.MessageI18NKey = "settings.hardening.check.behaviorModel.ok"
		return check
	}
	check.Status = "warning"
	check.Score = check.MaxScore / 2
	return check
}

func (h *HardeningHandler) GetBehaviorActivity(w http.ResponseWriter, r *http.Request) {
	if h.runtime == nil || h.behavior == nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	settings, err := h.runtime.GetRuntimeSettings(r.Context())
	if err != nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	if settings == nil || !settings.BehaviorModelEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
		})
		return
	}
	sess, _ := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	targetUserID := sess.UserID
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		if parsed, parseErr := strconv.ParseInt(raw, 10, 64); parseErr == nil && parsed > 0 {
			targetUserID = parsed
		}
	}
	now := time.Now().UTC()
	state, err := h.behavior.GetState(r.Context(), targetUserID)
	if err != nil || state == nil {
		http.Error(w, "common.serverError", http.StatusInternalServerError)
		return
	}
	count := func(d time.Duration, kinds ...string) int {
		n, cErr := h.behavior.CountEvents(r.Context(), targetUserID, now.Add(-d), kinds)
		if cErr != nil {
			return 0
		}
		return n
	}
	metrics := behavior.Metrics{
		SensitiveViews5m: count(5*time.Minute, "view.docs", "view.assets"),
		Exports30m:       count(30*time.Minute, "export.docs"),
		Denied10m:        count(10*time.Minute, "denied.security", "dlp.copy_blocked", "dlp.screenshot_attempt"),
		Mutations5m:      count(5*time.Minute, "mutation.high"),
		Requests1m:       count(time.Minute, "request.any"),
		HistorySensitive: count(7*24*time.Hour, "view.docs", "view.assets"),
		HistoryExports:   count(7*24*time.Hour, "export.docs"),
		HistoryDenied:    count(7*24*time.Hour, "denied.security", "dlp.copy_blocked", "dlp.screenshot_attempt"),
		HistoryMutations: count(7*24*time.Hour, "mutation.high"),
		HistoryRequests:  count(7*24*time.Hour, "request.any"),
		HistoryEvents:    count(7 * 24 * time.Hour),
		IPNovelty:        0,
		RecentlyVerified: state.LastVerifiedAt != nil && now.Sub(*state.LastVerifiedAt) <= 24*time.Hour,
	}
	dlpCopy10m := count(10*time.Minute, "dlp.copy_blocked")
	dlpScreenshot10m := count(10*time.Minute, "dlp.screenshot_attempt")
	dlpCopyHistory := count(7*24*time.Hour, "dlp.copy_blocked")
	dlpScreenshotHistory := count(7*24*time.Hour, "dlp.screenshot_attempt")
	eval := behavior.Evaluate(metrics)
	reasons := make([]map[string]any, 0, 5)
	addReason := func(key string, value float64, high bool) {
		reasons = append(reasons, map[string]any{
			"key":   key,
			"value": value,
			"high":  high,
		})
	}
	addReason("z_sensitive", eval.Features["z_sensitive"], eval.Features["z_sensitive"] >= 2.0)
	addReason("z_exports", eval.Features["z_exports"], metrics.Exports30m > 0 || eval.Features["z_exports"] >= 1.0)
	addReason("z_denied", eval.Features["z_denied"], metrics.Denied10m > 0 || eval.Features["z_denied"] >= 1.0)
	addReason("z_mutations", eval.Features["z_mutations"], eval.Features["z_mutations"] >= 2.0)
	addReason("z_requests", eval.Features["z_requests"], metrics.Requests1m >= 120 || eval.Features["z_requests"] >= 2.5)
	reasons = append(reasons, map[string]any{
		"key":   "dlp_copy",
		"value": float64(dlpCopy10m),
		"high":  dlpCopy10m >= 3,
	})
	reasons = append(reasons, map[string]any{
		"key":   "dlp_screenshot",
		"value": float64(dlpScreenshot10m),
		"high":  dlpScreenshot10m >= 2,
	})
	sort.Slice(reasons, func(i, j int) bool {
		return reasons[i]["value"].(float64) > reasons[j]["value"].(float64)
	})
	username := ""
	fullName := ""
	if h.users != nil {
		if u, _, uErr := h.users.Get(r.Context(), targetUserID); uErr == nil && u != nil {
			username = u.Username
			fullName = u.FullName
		}
	}
	suspicious := eval.Trigger || state.StepupRequired
	riskLevel := "low"
	switch {
	case eval.Score >= 0.88 || suspicious:
		riskLevel = "high"
	case eval.Score >= 0.7:
		riskLevel = "medium"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    true,
		"user_id":    targetUserID,
		"username":   username,
		"full_name":  fullName,
		"suspicious": suspicious,
		"risk_level": riskLevel,
		"score":      eval.Score,
		"triggered":  eval.Trigger,
		"state": map[string]any{
			"stepup_required":   state.StepupRequired,
			"failed_stepups":    state.FailedStepups,
			"last_risk_score":   state.LastRiskScore,
			"locked_until":      state.LockedUntil,
			"last_triggered_at": state.LastTriggeredAt,
			"last_verified_at":  state.LastVerifiedAt,
			"updated_at":        state.UpdatedAt,
		},
		"metrics": metrics,
		"dlp_signatures": map[string]any{
			"copy_blocked_10m":        dlpCopy10m,
			"screenshot_attempts_10m": dlpScreenshot10m,
			"copy_blocked_7d":         dlpCopyHistory,
			"screenshot_attempts_7d":  dlpScreenshotHistory,
		},
		"reasons": reasons,
	})
}

func (h *HardeningHandler) deploymentMode(r *http.Request) string {
	if h.runtime != nil {
		if item, err := h.runtime.GetRuntimeSettings(r.Context()); err == nil && item != nil {
			val := strings.ToLower(strings.TrimSpace(item.DeploymentMode))
			if val != "" {
				return val
			}
		}
	}
	if h.cfg != nil && h.cfg.IsHomeMode() {
		return "home"
	}
	return "enterprise"
}

func (h *HardeningHandler) httpsMode(r *http.Request) string {
	if isSecureRequest(r, h.cfg) {
		// TLS is effectively terminated (built-in or trusted reverse proxy).
		return HTTPSModeExternalProxy
	}
	if h.https != nil {
		if item, err := h.https.GetHTTPSSettings(r.Context()); err == nil && item != nil {
			if mode := strings.ToLower(strings.TrimSpace(item.Mode)); mode != "" {
				return mode
			}
		}
	}
	if h.cfg != nil && h.cfg.TLSEnabled {
		return HTTPSModeBuiltinTLS
	}
	return HTTPSModeDisabled
}

func (h *HardeningHandler) trustedProxies(r *http.Request) []string {
	if h.https != nil {
		if item, err := h.https.GetHTTPSSettings(r.Context()); err == nil && item != nil && len(item.TrustedProxies) > 0 {
			return item.TrustedProxies
		}
	}
	if h.cfg != nil {
		return append([]string(nil), h.cfg.Security.TrustedProxies...)
	}
	return nil
}

func isWeakSecret(value string) bool {
	val := strings.TrimSpace(value)
	if len(val) < 24 {
		return true
	}
	upper := strings.ToUpper(val)
	if strings.Contains(upper, "REPLACE_WITH") || strings.Contains(upper, "CHANGE_ME") || strings.Contains(upper, "EXAMPLE") {
		return true
	}
	return false
}

func isBroadCIDR(value string) bool {
	val := strings.TrimSpace(value)
	if val == "" || !strings.Contains(val, "/") {
		return false
	}
	prefix, err := netip.ParsePrefix(val)
	if err != nil {
		return false
	}
	bits := prefix.Bits()
	if prefix.Addr().Is4() {
		return bits <= 16
	}
	return bits <= 48
}
