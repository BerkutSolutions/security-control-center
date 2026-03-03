package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

type monitoringSettingsPayload struct {
	RetentionDays           int     `json:"retention_days"`
	MaxConcurrentChecks     int     `json:"max_concurrent_checks"`
	DefaultTimeoutSec       int     `json:"default_timeout_sec"`
	DefaultIntervalSec      int     `json:"default_interval_sec"`
	DefaultRetries          int     `json:"default_retries"`
	DefaultRetryIntervalSec int     `json:"default_retry_interval_sec"`
	DefaultSLATargetPct     float64 `json:"default_sla_target_pct"`
	EngineEnabled           *bool   `json:"engine_enabled"`
	AllowPrivateNetworks    *bool   `json:"allow_private_networks"`
	IssueEscalateMinutes    int     `json:"issue_escalate_minutes"`
	NotifyUpConfirmations   int     `json:"notify_up_confirmations"`
	TLSRefreshHours         int     `json:"tls_refresh_hours"`
	TLSExpiringDays         int     `json:"tls_expiring_days"`
	NotifySuppressMinutes   int     `json:"notify_suppress_minutes"`
	NotifyRepeatDownMinutes int     `json:"notify_repeat_down_minutes"`
	NotifyMaintenance       *bool   `json:"notify_maintenance"`
	LogDNSEvents            *bool   `json:"log_dns_events"`
	AutoTaskOnDown          *bool   `json:"auto_task_on_down"`
	AutoTLSIncident         *bool   `json:"auto_tls_incident"`
	AutoTLSIncidentDays     int     `json:"auto_tls_incident_days"`
	AutoIncidentCloseOnUp   *bool   `json:"auto_incident_close_on_up"`

	IncidentScoringEnabled         *bool   `json:"incident_scoring_enabled"`
	IncidentScoringModel           string  `json:"incident_scoring_model"`
	IncidentScoreOpenThreshold     float64 `json:"incident_score_open_threshold"`
	IncidentScoreCloseThreshold    float64 `json:"incident_score_close_threshold"`
	IncidentScoreOpenConfirmations int     `json:"incident_score_open_confirmations"`
}

func (h *MonitoringHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.settings.manage") {
		return
	}
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *MonitoringHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.settings.manage") {
		return
	}
	var payload monitoringSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	current, err := h.store.GetSettings(r.Context())
	if err != nil || current == nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	prevTLSRefresh := current.TLSRefreshHours
	prevTLSExpiring := current.TLSExpiringDays
	if payload.RetentionDays > 0 {
		current.RetentionDays = payload.RetentionDays
	}
	if payload.MaxConcurrentChecks > 0 {
		current.MaxConcurrentChecks = payload.MaxConcurrentChecks
	}
	if payload.DefaultTimeoutSec > 0 {
		current.DefaultTimeoutSec = payload.DefaultTimeoutSec
	}
	if payload.DefaultIntervalSec > 0 {
		current.DefaultIntervalSec = payload.DefaultIntervalSec
	}
	if payload.DefaultRetries >= 0 {
		current.DefaultRetries = payload.DefaultRetries
	}
	if payload.DefaultRetryIntervalSec > 0 {
		current.DefaultRetryIntervalSec = payload.DefaultRetryIntervalSec
	}
	if payload.DefaultSLATargetPct > 0 {
		current.DefaultSLATargetPct = payload.DefaultSLATargetPct
	}
	if payload.EngineEnabled != nil {
		current.EngineEnabled = *payload.EngineEnabled
	}
	if payload.AllowPrivateNetworks != nil {
		current.AllowPrivateNetworks = *payload.AllowPrivateNetworks
	}
	if payload.IssueEscalateMinutes > 0 {
		current.IssueEscalateMinutes = payload.IssueEscalateMinutes
	}
	if payload.NotifyUpConfirmations > 0 {
		current.NotifyUpConfirmations = payload.NotifyUpConfirmations
	}
	if payload.TLSRefreshHours > 0 {
		current.TLSRefreshHours = payload.TLSRefreshHours
	}
	if payload.TLSExpiringDays > 0 {
		current.TLSExpiringDays = payload.TLSExpiringDays
	}
	if payload.NotifySuppressMinutes > 0 {
		current.NotifySuppressMinutes = payload.NotifySuppressMinutes
	}
	if payload.NotifyRepeatDownMinutes > 0 {
		current.NotifyRepeatDownMinutes = payload.NotifyRepeatDownMinutes
	}
	if payload.NotifyMaintenance != nil {
		current.NotifyMaintenance = *payload.NotifyMaintenance
	}
	if payload.LogDNSEvents != nil {
		current.LogDNSEvents = *payload.LogDNSEvents
	}
	if payload.AutoTaskOnDown != nil {
		current.AutoTaskOnDown = *payload.AutoTaskOnDown
	}
	if payload.AutoTLSIncident != nil {
		current.AutoTLSIncident = *payload.AutoTLSIncident
	}
	if payload.AutoTLSIncidentDays > 0 {
		current.AutoTLSIncidentDays = payload.AutoTLSIncidentDays
	}
	if payload.AutoIncidentCloseOnUp != nil {
		current.AutoIncidentCloseOnUp = *payload.AutoIncidentCloseOnUp
	}
	if payload.IncidentScoringEnabled != nil {
		current.IncidentScoringEnabled = *payload.IncidentScoringEnabled
	}
	if strings.TrimSpace(payload.IncidentScoringModel) != "" {
		current.IncidentScoringModel = strings.ToLower(strings.TrimSpace(payload.IncidentScoringModel))
	}
	if payload.IncidentScoreOpenThreshold > 0 {
		current.IncidentScoreOpenThreshold = payload.IncidentScoreOpenThreshold
	}
	if payload.IncidentScoreCloseThreshold > 0 {
		current.IncidentScoreCloseThreshold = payload.IncidentScoreCloseThreshold
	}
	if payload.IncidentScoreOpenConfirmations > 0 {
		current.IncidentScoreOpenConfirmations = payload.IncidentScoreOpenConfirmations
	}
	if current.RetentionDays <= 0 || current.DefaultTimeoutSec <= 0 || current.DefaultIntervalSec <= 0 || current.MaxConcurrentChecks <= 0 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.DefaultRetries < 0 || current.DefaultRetries > 5 || current.DefaultRetryIntervalSec <= 0 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.DefaultSLATargetPct <= 0 || current.DefaultSLATargetPct > 100 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.TLSRefreshHours <= 0 || current.TLSExpiringDays <= 0 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IssueEscalateMinutes < 1 || current.IssueEscalateMinutes > 24*60 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.NotifyUpConfirmations < 1 || current.NotifyUpConfirmations > 10 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.NotifySuppressMinutes < 0 || current.NotifyRepeatDownMinutes < 0 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.AutoTLSIncidentDays <= 0 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IncidentScoreOpenThreshold < 0 || current.IncidentScoreOpenThreshold > 1 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IncidentScoringModel == "" {
		current.IncidentScoringModel = "heuristic"
	}
	if current.IncidentScoringModel != "heuristic" && current.IncidentScoringModel != "hmm3" {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IncidentScoreCloseThreshold < 0 || current.IncidentScoreCloseThreshold > 1 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IncidentScoreCloseThreshold >= current.IncidentScoreOpenThreshold {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if current.IncidentScoreOpenConfirmations < 1 || current.IncidentScoreOpenConfirmations > 10 {
		http.Error(w, "monitoring.error.invalidSettings", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateSettings(r.Context(), current); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if h.engine != nil {
		h.engine.InvalidateSettings()
	}
	h.audit(r, monitorAuditSettingsUpdate, settingsDetails(current))
	if prevTLSRefresh != current.TLSRefreshHours || prevTLSExpiring != current.TLSExpiringDays {
		h.audit(r, monitorAuditCertsSettingsUpdate, settingsDetails(current))
	}
	h.audit(r, monitorAuditIncidentScoringUpdate, settingsDetails(current))
	writeJSON(w, http.StatusOK, current)
}

func settingsDetails(s *store.MonitorSettings) string {
	if s == nil {
		return ""
	}
	parts := []string{
		"retention=" + strconv.Itoa(s.RetentionDays),
		"max_concurrent=" + strconv.Itoa(s.MaxConcurrentChecks),
		"default_timeout=" + strconv.Itoa(s.DefaultTimeoutSec),
		"default_interval=" + strconv.Itoa(s.DefaultIntervalSec),
		"default_retries=" + strconv.Itoa(s.DefaultRetries),
		"default_retry_interval=" + strconv.Itoa(s.DefaultRetryIntervalSec),
		"default_sla=" + strconv.FormatFloat(s.DefaultSLATargetPct, 'f', 1, 64),
		"engine=" + strconv.FormatBool(s.EngineEnabled),
		"allow_private=" + strconv.FormatBool(s.AllowPrivateNetworks),
		"issue_escalate_minutes=" + strconv.Itoa(s.IssueEscalateMinutes),
		"notify_up_confirmations=" + strconv.Itoa(s.NotifyUpConfirmations),
		"tls_refresh=" + strconv.Itoa(s.TLSRefreshHours),
		"tls_expiring=" + strconv.Itoa(s.TLSExpiringDays),
		"notify_suppress=" + strconv.Itoa(s.NotifySuppressMinutes),
		"notify_repeat=" + strconv.Itoa(s.NotifyRepeatDownMinutes),
		"notify_maintenance=" + strconv.FormatBool(s.NotifyMaintenance),
		"log_dns_events=" + strconv.FormatBool(s.LogDNSEvents),
		"auto_task_on_down=" + strconv.FormatBool(s.AutoTaskOnDown),
		"auto_tls_incident=" + strconv.FormatBool(s.AutoTLSIncident),
		"auto_tls_incident_days=" + strconv.Itoa(s.AutoTLSIncidentDays),
		"auto_incident_close_on_up=" + strconv.FormatBool(s.AutoIncidentCloseOnUp),
		"incident_scoring_enabled=" + strconv.FormatBool(s.IncidentScoringEnabled),
		"incident_scoring_model=" + s.IncidentScoringModel,
		"incident_score_open_threshold=" + strconv.FormatFloat(s.IncidentScoreOpenThreshold, 'f', 4, 64),
		"incident_score_close_threshold=" + strconv.FormatFloat(s.IncidentScoreCloseThreshold, 'f', 4, 64),
		"incident_score_open_confirmations=" + strconv.Itoa(s.IncidentScoreOpenConfirmations),
	}
	return strings.Join(parts, "|")
}
