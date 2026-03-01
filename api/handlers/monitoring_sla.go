package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

type slaWindowView struct {
	UptimePct   float64 `json:"uptime_pct"`
	CoveragePct float64 `json:"coverage_pct"`
	TargetPct   float64 `json:"target_pct"`
	Status      string  `json:"status"`
}

type monitorSLAOverviewItem struct {
	MonitorID int64                  `json:"monitor_id"`
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	CreatedAt time.Time              `json:"created_at"`
	IsActive  bool                   `json:"is_active"`
	IsPaused  bool                   `json:"is_paused"`
	Status    string                 `json:"status"`
	TargetPct float64                `json:"target_pct"`
	Policy    store.MonitorSLAPolicy `json:"policy"`
	Window24h slaWindowView          `json:"window_24h"`
	Window7d  slaWindowView          `json:"window_7d"`
	Window30d slaWindowView          `json:"window_30d"`
}

type monitorSLAPolicyPayload struct {
	IncidentOnViolation bool     `json:"incident_on_violation"`
	IncidentPeriod      string   `json:"incident_period"`
	MinCoveragePct      *float64 `json:"min_coverage_pct"`
}

func (h *MonitoringHandler) ListSLAOverview(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.view") {
		return
	}
	monitors, err := h.store.ListMonitors(r.Context(), store.MonitorFilter{})
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	settings, err := h.store.GetSettings(r.Context())
	if err != nil || settings == nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	ids := make([]int64, 0, len(monitors))
	for _, mon := range monitors {
		ids = append(ids, mon.ID)
	}
	policies, err := h.store.ListMonitorSLAPolicies(r.Context(), ids)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	policyMap := make(map[int64]store.MonitorSLAPolicy, len(policies))
	for _, item := range policies {
		policyMap[item.MonitorID] = item
	}
	now := time.Now().UTC()
	filterStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if filterStatus != "ok" && filterStatus != "violated" && filterStatus != "unknown" {
		filterStatus = ""
	}
	var items []monitorSLAOverviewItem
	for _, mon := range monitors {
		policy := defaultPolicy(mon.ID)
		if existing, ok := policyMap[mon.ID]; ok {
			policy = existing
		}
		target := effectiveTarget(mon.Monitor, *settings)
		w24, err := h.evaluateSLAWindow(r, mon.Monitor, policy, *settings, now.Add(-24*time.Hour), now)
		if err != nil {
			http.Error(w, errServerError, http.StatusInternalServerError)
			return
		}
		w7, err := h.evaluateSLAWindow(r, mon.Monitor, policy, *settings, now.Add(-7*24*time.Hour), now)
		if err != nil {
			http.Error(w, errServerError, http.StatusInternalServerError)
			return
		}
		w30, err := h.evaluateSLAWindow(r, mon.Monitor, policy, *settings, now.Add(-30*24*time.Hour), now)
		if err != nil {
			http.Error(w, errServerError, http.StatusInternalServerError)
			return
		}
		if filterStatus != "" && w30.Status != filterStatus {
			continue
		}
		items = append(items, monitorSLAOverviewItem{
			MonitorID: mon.ID,
			Name:      mon.Name,
			Type:      mon.Type,
			CreatedAt: mon.CreatedAt,
			IsActive:  mon.IsActive,
			IsPaused:  mon.IsPaused,
			Status:    mon.Status,
			TargetPct: target,
			Policy:    policy,
			Window24h: w24,
			Window7d:  w7,
			Window30d: w30,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) ListSLAHistory(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.view") {
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	filter := store.MonitorSLAPeriodResultListFilter{
		Limit:      limit,
		PeriodType: strings.TrimSpace(r.URL.Query().Get("period")),
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
	}
	onlyViolations := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("only_violations")))
	filter.OnlyViolates = onlyViolations == "1" || onlyViolations == "true"
	items, err := h.store.ListSLAPeriodResults(r.Context(), filter)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	monitors, err := h.store.ListMonitors(r.Context(), store.MonitorFilter{})
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	names := make(map[int64]string, len(monitors))
	for _, mon := range monitors {
		names[mon.ID] = mon.Name
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]any{
			"id":               item.ID,
			"monitor_id":       item.MonitorID,
			"monitor_name":     names[item.MonitorID],
			"period_type":      item.PeriodType,
			"period_start":     item.PeriodStart,
			"period_end":       item.PeriodEnd,
			"uptime_pct":       item.UptimePct,
			"coverage_pct":     item.CoveragePct,
			"target_pct":       item.TargetPct,
			"status":           item.Status,
			"incident_created": item.IncidentCreated,
			"created_at":       item.CreatedAt,
			"updated_at":       item.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *MonitoringHandler) UpdateMonitorSLAPolicy(w http.ResponseWriter, r *http.Request) {
	if !h.requirePerm(w, r, "monitoring.manage") {
		return
	}
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	mon, err := h.store.GetMonitor(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if mon == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	var payload monitorSLAPolicyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	current, err := h.store.GetMonitorSLAPolicy(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	period := strings.ToLower(strings.TrimSpace(payload.IncidentPeriod))
	if period != "day" && period != "week" && period != "month" {
		http.Error(w, "monitoring.sla.error.invalidIncidentPeriod", http.StatusBadRequest)
		return
	}
	if payload.IncidentOnViolation && !hasPermission(r, h.policy, "monitoring.incidents.link") {
		http.Error(w, "monitoring.forbiddenIncidentLink", http.StatusForbidden)
		return
	}
	minCoverage := 80.0
	if current != nil && current.MinCoveragePct > 0 && current.MinCoveragePct <= 100 {
		minCoverage = current.MinCoveragePct
	}
	if payload.MinCoveragePct != nil {
		if *payload.MinCoveragePct <= 0 || *payload.MinCoveragePct > 100 {
			http.Error(w, "monitoring.sla.error.invalidCoverage", http.StatusBadRequest)
			return
		}
		minCoverage = round2(*payload.MinCoveragePct)
	}
	item := &store.MonitorSLAPolicy{
		MonitorID:           id,
		IncidentOnViolation: payload.IncidentOnViolation,
		IncidentPeriod:      period,
		MinCoveragePct:      minCoverage,
	}
	if err := h.store.UpsertMonitorSLAPolicy(r.Context(), item); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditSLAPolicyUpdate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, item)
}

func (h *MonitoringHandler) evaluateSLAWindow(r *http.Request, monitor store.Monitor, policy store.MonitorSLAPolicy, settings store.MonitorSettings, since, until time.Time) (slaWindowView, error) {
	target := effectiveTarget(monitor, settings)
	if h.engine != nil {
		eval, err := h.engine.EvaluateMonitorSLAWindow(r.Context(), monitor, policy, settings, since, until)
		if err != nil {
			return slaWindowView{}, err
		}
		return slaWindowView{
			UptimePct:   eval.UptimePct,
			CoveragePct: eval.CoveragePct,
			TargetPct:   eval.TargetPct,
			Status:      eval.Status,
		}, nil
	}
	return slaWindowView{
		UptimePct:   0,
		CoveragePct: 0,
		TargetPct:   round2(target),
		Status:      "unknown",
	}, nil
}

func effectiveTarget(mon store.Monitor, settings store.MonitorSettings) float64 {
	if mon.SLATargetPct != nil && *mon.SLATargetPct > 0 && *mon.SLATargetPct <= 100 {
		return *mon.SLATargetPct
	}
	if settings.DefaultSLATargetPct > 0 && settings.DefaultSLATargetPct <= 100 {
		return settings.DefaultSLATargetPct
	}
	return 90
}

func defaultPolicy(monitorID int64) store.MonitorSLAPolicy {
	return store.MonitorSLAPolicy{
		MonitorID:           monitorID,
		IncidentOnViolation: false,
		IncidentPeriod:      "day",
		MinCoveragePct:      80,
		UpdatedAt:           time.Now().UTC(),
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
