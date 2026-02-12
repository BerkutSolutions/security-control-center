package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

func (h *MonitoringHandler) ListCerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.CertFilter{
		Tags:   splitCSV(q.Get("tag")),
		Status: strings.TrimSpace(q.Get("status")),
	}
	if val := strings.TrimSpace(q.Get("expiring_lt")); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			filter.ExpiringLt = n
		}
	}
	items, err := h.store.ListCerts(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	settings, _ := h.store.GetSettings(r.Context())
	threshold := 30
	if settings != nil && settings.TLSExpiringDays > 0 {
		threshold = settings.TLSExpiringDays
	}
	for i := range items {
		if items[i].DaysLeft != nil && *items[i].DaysLeft <= threshold {
			items[i].ExpiringSoon = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) TestCertNotification(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload struct {
		MonitorIDs []int64 `json:"monitor_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(payload.MonitorIDs) == 0 {
		http.Error(w, "monitoring.certs.notifyNoMonitors", http.StatusBadRequest)
		return
	}
	for _, id := range payload.MonitorIDs {
		if err := h.engine.TestTLSNotification(r.Context(), id); err != nil {
			h.audits.Log(r.Context(), currentUsername(r), "monitoring.certs.notify_test.failed", strconv.FormatInt(id, 10))
			http.Error(w, "monitoring.notifications.testFailed", http.StatusBadRequest)
			return
		}
	}
	h.audits.Log(r.Context(), currentUsername(r), "monitoring.certs.notify_test", strings.TrimSpace(strings.Join(int64SliceToStrings(payload.MonitorIDs), ",")))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func int64SliceToStrings(items []int64) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, id := range items {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out
}

func (h *MonitoringHandler) GetTLS(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetTLS(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
