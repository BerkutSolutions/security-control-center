package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/appmeta"
	"berkut-scc/core/store"
)

type RuntimeSettingsHandler struct {
	cfg          *config.AppConfig
	runtimeStore store.AppRuntimeStore
	updateCheck  *appmeta.UpdateChecker
	audits       store.AuditStore
}

func NewRuntimeSettingsHandler(cfg *config.AppConfig, runtimeStore store.AppRuntimeStore, updateCheck *appmeta.UpdateChecker, audits store.AuditStore) *RuntimeSettingsHandler {
	return &RuntimeSettingsHandler{
		cfg:          cfg,
		runtimeStore: runtimeStore,
		updateCheck:  updateCheck,
		audits:       audits,
	}
}

func (h *RuntimeSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, h.responsePayload(settings, h.lastUpdateResult()))
}

func (h *RuntimeSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	beforeMode := settings.DeploymentMode
	beforeUpdates := settings.UpdateChecksEnabled

	var payload struct {
		DeploymentMode      string `json:"deployment_mode"`
		UpdateChecksEnabled *bool  `json:"update_checks_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	mode := normalizeDeploymentMode(payload.DeploymentMode)
	if mode == "" {
		mode = settings.DeploymentMode
	}
	settings.DeploymentMode = mode
	if payload.UpdateChecksEnabled != nil {
		settings.UpdateChecksEnabled = *payload.UpdateChecksEnabled
	}
	if err := h.runtimeStore.SaveRuntimeSettings(r.Context(), settings); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.cfg != nil {
		h.cfg.DeploymentMode = settings.DeploymentMode
		if settings.DeploymentMode == "home" {
			h.cfg.TLSEnabled = false
		}
	}
	if beforeMode != settings.DeploymentMode {
		_ = h.audits.Log(r.Context(), currentUsername(r), "settings.deployment_mode.update", "mode="+settings.DeploymentMode)
	}
	if beforeUpdates != settings.UpdateChecksEnabled {
		_ = h.audits.Log(r.Context(), currentUsername(r), "settings.updates.toggle", "enabled="+strconv.FormatBool(settings.UpdateChecksEnabled))
	}
	writeJSON(w, http.StatusOK, h.responsePayload(settings, h.lastUpdateResult()))
}

func (h *RuntimeSettingsHandler) CheckUpdates(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !settings.UpdateChecksEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"result":  h.lastUpdateResult(),
		})
		return
	}
	result, err := h.checkUpdatesSafe(r)
	if err != nil {
		http.Error(w, "update check failed", http.StatusBadGateway)
		return
	}
	details := strings.Join([]string{
		"current=" + appmeta.AppVersion,
		"latest=" + result.LatestVersion,
		"has_update=" + strconv.FormatBool(result.HasUpdate),
	}, "|")
	_ = h.audits.Log(r.Context(), currentUsername(r), "settings.updates.check", details)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": true,
		"result":  result,
	})
}

func (h *RuntimeSettingsHandler) Meta(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	result := h.lastUpdateResult()
	if settings.UpdateChecksEnabled {
		before := h.lastUpdateResult()
		if checked, err := h.checkUpdatesSafe(r); err == nil && checked != nil {
			result = checked
			if before == nil || checked.CheckedAt.After(before.CheckedAt) {
				details := strings.Join([]string{
					"current=" + appmeta.AppVersion,
					"latest=" + checked.LatestVersion,
					"has_update=" + strconv.FormatBool(checked.HasUpdate),
					"source=meta",
				}, "|")
				_ = h.audits.Log(r.Context(), currentUsername(r), "settings.updates.check", details)
			}
		}
	}
	writeJSON(w, http.StatusOK, h.responsePayload(settings, result))
}

func (h *RuntimeSettingsHandler) loadSettings(r *http.Request) (*store.AppRuntimeSettings, error) {
	settings, err := h.runtimeStore.GetRuntimeSettings(r.Context())
	if err != nil {
		return nil, err
	}
	if settings == nil {
		settings = &store.AppRuntimeSettings{
			DeploymentMode:      effectiveMode(h.cfg, nil),
			UpdateChecksEnabled: false,
		}
	}
	settings.DeploymentMode = effectiveMode(h.cfg, settings)
	return settings, nil
}

func (h *RuntimeSettingsHandler) responsePayload(settings *store.AppRuntimeSettings, result *appmeta.UpdateCheckResult) map[string]any {
	mode := effectiveMode(h.cfg, settings)
	return map[string]any{
		"app_version":           appmeta.AppVersion,
		"repository_url":        appmeta.RepositoryURL,
		"deployment_mode":       mode,
		"is_home_mode":          mode == "home",
		"update_checks_enabled": settings.UpdateChecksEnabled,
		"update":                result,
	}
}

func (h *RuntimeSettingsHandler) lastUpdateResult() *appmeta.UpdateCheckResult {
	if h.updateCheck == nil {
		return nil
	}
	return h.updateCheck.LastResult()
}

func (h *RuntimeSettingsHandler) checkUpdatesSafe(r *http.Request) (*appmeta.UpdateCheckResult, error) {
	if h.updateCheck == nil {
		return &appmeta.UpdateCheckResult{
			CurrentVersion: appmeta.AppVersion,
			LatestVersion:  appmeta.AppVersion,
			ReleaseURL:     appmeta.RepositoryURL,
			HasUpdate:      false,
			Source:         "disabled",
		}, nil
	}
	result, err := h.updateCheck.Check(r.Context(), appmeta.AppVersion)
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	last := h.updateCheck.LastResult()
	if last != nil {
		return last, nil
	}
	return &appmeta.UpdateCheckResult{
		CurrentVersion: appmeta.AppVersion,
		LatestVersion:  appmeta.AppVersion,
		ReleaseURL:     appmeta.RepositoryURL,
		HasUpdate:      false,
		Source:         "unknown",
	}, nil
}

func effectiveMode(cfg *config.AppConfig, settings *store.AppRuntimeSettings) string {
	if settings != nil && settings.DeploymentMode != "" {
		return normalizeDeploymentMode(settings.DeploymentMode)
	}
	if cfg != nil && cfg.IsHomeMode() {
		return "home"
	}
	return "enterprise"
}

func normalizeDeploymentMode(mode string) string {
	val := strings.ToLower(strings.TrimSpace(mode))
	switch val {
	case "home":
		return "home"
	case "enterprise":
		return "enterprise"
	default:
		return ""
	}
}
