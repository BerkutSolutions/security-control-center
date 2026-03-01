package handlers

import "net/http"

func (h *MonitoringHandler) GetEngineStats(w http.ResponseWriter, r *http.Request) {
	if !hasPermission(r, h.policy, "monitoring.view") {
		http.Error(w, errForbidden, http.StatusForbidden)
		return
	}
	if h == nil || h.engine == nil {
		http.Error(w, errServiceUnavailable, http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, h.engine.StatsSnapshot())
}

