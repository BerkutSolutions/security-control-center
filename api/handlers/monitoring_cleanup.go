package handlers

import (
	"net/http"
	"strconv"
)

func (h *MonitoringHandler) DeleteMonitorEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	deleted, err := h.store.DeleteMonitorEvents(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMonitorEventsDelete, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (h *MonitoringHandler) DeleteMonitorMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	deleted, err := h.store.DeleteMonitorMetrics(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMonitorMetricsDelete, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}
