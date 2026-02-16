package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/store"
)

const (
	HTTPSModeDisabled      = "disabled"
	HTTPSModeExternalProxy = "external_proxy"
	HTTPSModeBuiltinTLS    = "builtin_tls"
)

type HTTPSSettingsHandler struct {
	cfg    *config.AppConfig
	store  store.AppHTTPSStore
	audits store.AuditStore
}

func NewHTTPSSettingsHandler(cfg *config.AppConfig, st store.AppHTTPSStore, audits store.AuditStore) *HTTPSSettingsHandler {
	return &HTTPSSettingsHandler{cfg: cfg, store: st, audits: audits}
}

func (h *HTTPSSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetHTTPSSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		item = h.defaultSettings()
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *HTTPSSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var payload store.HTTPSSettings
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode != HTTPSModeDisabled && mode != HTTPSModeExternalProxy && mode != HTTPSModeBuiltinTLS {
		http.Error(w, "settings.https.invalidMode", http.StatusBadRequest)
		return
	}
	if payload.ListenPort <= 0 || payload.ListenPort > 65535 {
		http.Error(w, "settings.https.invalidPort", http.StatusBadRequest)
		return
	}
	payload.Mode = mode
	payload.BuiltinCertPath = strings.TrimSpace(payload.BuiltinCertPath)
	payload.BuiltinKeyPath = strings.TrimSpace(payload.BuiltinKeyPath)
	payload.ExternalProxyHint = strings.TrimSpace(payload.ExternalProxyHint)
	payload.TrustedProxies = sanitizeTrustedProxies(payload.TrustedProxies)
	if payload.Mode == HTTPSModeBuiltinTLS {
		if payload.BuiltinCertPath == "" || payload.BuiltinKeyPath == "" {
			http.Error(w, "settings.https.certRequired", http.StatusBadRequest)
			return
		}
	}
	existing, err := h.store.GetHTTPSSettings(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		payload.ID = existing.ID
	}
	if err := h.store.SaveHTTPSSettings(r.Context(), &payload); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	details := strings.Join([]string{
		"mode=" + payload.Mode,
		"port=" + strconv.Itoa(payload.ListenPort),
		"trusted_proxies=" + strings.Join(payload.TrustedProxies, ","),
		"external_proxy_hint=" + payload.ExternalProxyHint,
		"builtin_cert_set=" + strconv.FormatBool(payload.BuiltinCertPath != ""),
		"builtin_key_set=" + strconv.FormatBool(payload.BuiltinKeyPath != ""),
	}, "|")
	_ = h.audits.Log(r.Context(), currentUsername(r), "settings.https.update", details)
	writeJSON(w, http.StatusOK, payload)
}

func (h *HTTPSSettingsHandler) defaultSettings() *store.HTTPSSettings {
	listenAddr := "0.0.0.0:8080"
	trustedProxies := []string{}
	certPath := ""
	keyPath := ""
	mode := HTTPSModeDisabled
	if h.cfg != nil && h.cfg.TLSEnabled {
		mode = HTTPSModeBuiltinTLS
	}
	if envMode := strings.ToLower(strings.TrimSpace(os.Getenv("HTTPS_MODE"))); envMode != "" {
		switch envMode {
		case HTTPSModeDisabled, HTTPSModeExternalProxy, HTTPSModeBuiltinTLS:
			mode = envMode
		}
	}
	if h.cfg != nil {
		listenAddr = h.cfg.ListenAddr
		trustedProxies = append([]string(nil), h.cfg.Security.TrustedProxies...)
		certPath = strings.TrimSpace(h.cfg.TLSCert)
		keyPath = strings.TrimSpace(h.cfg.TLSKey)
	}
	port := extractPort(listenAddr)
	if port == 0 {
		port = 8080
	}
	if envPort := strings.TrimSpace(os.Getenv("HTTPS_PORT")); envPort != "" {
		if parsed, err := strconv.Atoi(envPort); err == nil && parsed > 0 && parsed <= 65535 {
			port = parsed
		}
	}
	if envTrusted := strings.TrimSpace(os.Getenv("HTTPS_TRUSTED_PROXIES")); envTrusted != "" {
		trustedProxies = splitCSV(envTrusted)
	}
	externalProxyHint := "nginx"
	if envHint := strings.TrimSpace(os.Getenv("HTTPS_EXTERNAL_PROXY_HINT")); envHint != "" {
		externalProxyHint = envHint
	}
	if envCert := strings.TrimSpace(os.Getenv("HTTPS_BUILTIN_CERT_PATH")); envCert != "" {
		certPath = envCert
	}
	if envKey := strings.TrimSpace(os.Getenv("HTTPS_BUILTIN_KEY_PATH")); envKey != "" {
		keyPath = envKey
	}
	return &store.HTTPSSettings{
		Mode:              mode,
		ListenPort:        port,
		TrustedProxies:    trustedProxies,
		BuiltinCertPath:   certPath,
		BuiltinKeyPath:    keyPath,
		ExternalProxyHint: externalProxyHint,
	}
}

func extractPort(listenAddr string) int {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil {
		return 0
	}
	_ = host
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

func sanitizeTrustedProxies(input []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(input))
	for _, item := range input {
		val := strings.TrimSpace(item)
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}
