package handlers

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
)

type HardeningHandler struct {
	cfg     *config.AppConfig
	https   store.AppHTTPSStore
	runtime store.AppRuntimeStore
	audits  store.AuditStore
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

func NewHardeningHandler(cfg *config.AppConfig, https store.AppHTTPSStore, runtime store.AppRuntimeStore, audits store.AuditStore) *HardeningHandler {
	return &HardeningHandler{cfg: cfg, https: https, runtime: runtime, audits: audits}
}

func (h *HardeningHandler) GetBaseline(w http.ResponseWriter, r *http.Request) {
	report := map[string]any{
		"generated_at": time.Now().UTC(),
		"score":        0,
		"max_score":    0,
		"status":       "warning",
		"checks":       []hardeningCheck{},
	}
	checks := make([]hardeningCheck, 0, 5)
	checks = append(checks, h.checkTLSMode(r))
	checks = append(checks, h.checkTrustedProxies(r))
	checks = append(checks, h.checkSecretsHealth())
	checks = append(checks, h.checkSessionPolicy())
	checks = append(checks, h.checkPasswordPolicy())
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
