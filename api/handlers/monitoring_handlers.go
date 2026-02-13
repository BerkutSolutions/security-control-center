package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/monitoring"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type MonitoringHandler struct {
	store     store.MonitoringStore
	audits    store.AuditStore
	engine    *monitoring.Engine
	policy    *rbac.Policy
	encryptor *utils.Encryptor
}

func NewMonitoringHandler(store store.MonitoringStore, audits store.AuditStore, engine *monitoring.Engine, policy *rbac.Policy, encryptor *utils.Encryptor) *MonitoringHandler {
	return &MonitoringHandler{store: store, audits: audits, engine: engine, policy: policy, encryptor: encryptor}
}

func (h *MonitoringHandler) ListMonitors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.MonitorFilter{
		Query:  strings.TrimSpace(q.Get("q")),
		Status: strings.TrimSpace(q.Get("status")),
		Tags:   splitCSV(q.Get("tag")),
	}
	if active := strings.TrimSpace(q.Get("active")); active != "" {
		val := active == "1" || strings.ToLower(active) == "true"
		filter.Active = &val
	}
	items, err := h.store.ListMonitors(r.Context(), filter)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	var payload monitorPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if requiresIncidentLink(payload) && !hasPermission(r, h.policy, "monitoring.incidents.link") {
		http.Error(w, "monitoring.forbiddenIncidentLink", http.StatusForbidden)
		return
	}
	settings, _ := h.store.GetSettings(r.Context())
	mon, err := payloadToMonitor(payload, settings, sessionUserID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := h.store.CreateMonitor(r.Context(), mon)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	mon.ID = id
	_ = h.store.UpsertMonitorState(r.Context(), &store.MonitorState{
		MonitorID:        mon.ID,
		Status:           initialStatus(mon.IsPaused),
		LastResultStatus: "down",
	})
	h.audit(r, monitorAuditMonitorCreate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, mon)
}

func (h *MonitoringHandler) GetMonitor(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, mon)
}

func (h *MonitoringHandler) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetMonitor(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	var payload monitorPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if requiresIncidentLink(payload) && !hasPermission(r, h.policy, "monitoring.incidents.link") {
		http.Error(w, "monitoring.forbiddenIncidentLink", http.StatusForbidden)
		return
	}
	slaChanged := false
	if payload.SLATargetPct != nil {
		if existing.SLATargetPct == nil || *existing.SLATargetPct != *payload.SLATargetPct {
			slaChanged = true
		}
	}
	settings, _ := h.store.GetSettings(r.Context())
	mon, err := mergeMonitor(existing, payload, settings)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateMonitor(r.Context(), mon); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if existing.IsPaused != mon.IsPaused {
		_ = h.store.SetMonitorPaused(r.Context(), id, mon.IsPaused)
	}
	h.audit(r, monitorAuditMonitorUpdate, strconv.FormatInt(id, 10))
	if slaChanged {
		h.audit(r, monitorAuditSLAUpdate, strconv.FormatInt(id, 10))
	}
	writeJSON(w, http.StatusOK, mon)
}

func (h *MonitoringHandler) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteMonitor(r.Context(), id); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMonitorDelete, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) PauseMonitor(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, true, monitorAuditMonitorPause)
}

func (h *MonitoringHandler) ResumeMonitor(w http.ResponseWriter, r *http.Request) {
	h.setPaused(w, r, false, monitorAuditMonitorResume)
}

func (h *MonitoringHandler) setPaused(w http.ResponseWriter, r *http.Request, paused bool, audit string) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if err := h.store.SetMonitorPaused(r.Context(), id, paused); err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, audit, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) CheckNow(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if h.engine == nil {
		http.Error(w, errServiceUnavailable, http.StatusServiceUnavailable)
		return
	}
	if err := h.engine.CheckNow(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.audit(r, monitorAuditMonitorCheckNow, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) CloneMonitor(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetMonitor(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	clone := *existing
	clone.ID = 0
	clone.Name = strings.TrimSpace(existing.Name) + " (copy)"
	clone.CreatedBy = sessionUserID(r)
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	newID, err := h.store.CreateMonitor(r.Context(), &clone)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	clone.ID = newID
	_ = h.store.UpsertMonitorState(r.Context(), &store.MonitorState{
		MonitorID:        clone.ID,
		Status:           initialStatus(clone.IsPaused),
		LastResultStatus: "down",
	})
	h.audit(r, monitorAuditMonitorClone, strconv.FormatInt(newID, 10))
	writeJSON(w, http.StatusCreated, clone)
}
