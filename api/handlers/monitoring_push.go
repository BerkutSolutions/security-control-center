package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/store"
)

type monitorPushPayload struct {
	OK         *bool  `json:"ok"`
	Error      string `json:"error"`
	StatusCode *int   `json:"status_code"`
	LatencyMS  *int   `json:"latency_ms"`
}

func (h *MonitoringHandler) PushMonitor(w http.ResponseWriter, r *http.Request) {
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
	if !monitoring.TypeIsPassive(mon.Type) {
		http.Error(w, "monitoring.error.pushOnly", http.StatusBadRequest)
		return
	}
	var payload monitorPushPayload
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, errBadRequest, http.StatusBadRequest)
			return
		}
	}
	ok := true
	if payload.OK != nil {
		ok = *payload.OK
	}
	now := time.Now().UTC()
	latency := 0
	if payload.LatencyMS != nil && *payload.LatencyMS > 0 {
		latency = *payload.LatencyMS
	}
	var errRef *string
	if payload.Error != "" {
		msg := payload.Error
		errRef = &msg
	}
	_, _ = h.store.AddMetric(r.Context(), &store.MonitorMetric{
		MonitorID:  mon.ID,
		TS:         now,
		LatencyMs:  latency,
		OK:         ok,
		StatusCode: payload.StatusCode,
		Error:      errRef,
	})
	prev, _ := h.store.GetMonitorState(r.Context(), mon.ID)
	rawStatus := "down"
	if ok {
		rawStatus = "up"
	}
	maintenanceActive := false
	if list, err := h.store.ActiveMaintenanceFor(r.Context(), mon.ID, mon.Tags, now); err == nil && len(list) > 0 {
		maintenanceActive = true
	}
	status := rawStatus
	if mon.IsPaused {
		status = "paused"
	} else if maintenanceActive {
		status = "maintenance"
	}
	lastError := payload.Error
	if ok {
		lastError = ""
	}
	next := &store.MonitorState{
		MonitorID:         mon.ID,
		Status:            status,
		LastResultStatus:  rawStatus,
		MaintenanceActive: maintenanceActive,
		LastCheckedAt:     &now,
		LastStatusCode:    payload.StatusCode,
		LastError:         lastError,
	}
	if latency > 0 {
		next.LastLatencyMs = &latency
	}
	if rawStatus == "up" {
		next.LastUpAt = &now
	} else {
		next.LastDownAt = &now
	}
	if prev != nil {
		if next.LastUpAt == nil {
			next.LastUpAt = prev.LastUpAt
		}
		if next.LastDownAt == nil {
			next.LastDownAt = prev.LastDownAt
		}
	}
	ok24, total24, avg24, _ := h.store.MetricsSummary(r.Context(), mon.ID, now.Add(-24*time.Hour))
	ok30, total30, _, _ := h.store.MetricsSummary(r.Context(), mon.ID, now.Add(-30*24*time.Hour))
	if total24 > 0 {
		next.Uptime24h = (float64(ok24) / float64(total24)) * 100
	}
	if total30 > 0 {
		next.Uptime30d = (float64(ok30) / float64(total30)) * 100
	}
	next.AvgLatency24h = avg24
	if prev == nil || prev.LastResultStatus != rawStatus {
		msg := payload.Error
		if msg == "" && payload.StatusCode != nil {
			msg = "status_" + strconv.Itoa(*payload.StatusCode)
		}
		_, _ = h.store.AddEvent(r.Context(), &store.MonitorEvent{
			MonitorID: mon.ID,
			TS:        now,
			EventType: rawStatus,
			Message:   msg,
		})
	}
	if err := h.store.UpsertMonitorState(r.Context(), next); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMonitorPush, strconv.FormatInt(mon.ID, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
