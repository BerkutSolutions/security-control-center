package handlers

import (
	"net/http"
	"time"

	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

func (h *MonitoringHandler) GetState(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	state, err := h.store.GetMonitorState(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if state == nil {
		state = &store.MonitorState{MonitorID: id, Status: "down", LastResultStatus: "down"}
	}
	now := time.Now().UTC()
	ok24, total24, avg24, _ := h.store.MetricsSummary(r.Context(), id, now.Add(-24*time.Hour))
	ok30, total30, _, _ := h.store.MetricsSummary(r.Context(), id, now.Add(-30*24*time.Hour))
	if total24 > 0 {
		state.Uptime24h = (float64(ok24) / float64(total24)) * 100
	}
	if total30 > 0 {
		state.Uptime30d = (float64(ok30) / float64(total30)) * 100
	}
	state.AvgLatency24h = avg24
	writeJSON(w, http.StatusOK, state)
}

func (h *MonitoringHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rng := r.URL.Query().Get("range")
	since := rangeSince(rng, 24*time.Hour)
	items, err := h.store.ListMetrics(r.Context(), id, since)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"from":  since,
		"to":    time.Now().UTC(),
	})
}

func (h *MonitoringHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rng := r.URL.Query().Get("range")
	since := rangeSince(rng, 24*time.Hour)
	items, err := h.store.ListEvents(r.Context(), id, since)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"from":  since,
		"to":    time.Now().UTC(),
	})
}

func rangeSince(raw string, fallback time.Duration) time.Time {
	switch raw {
	case "1h":
		return time.Now().UTC().Add(-1 * time.Hour)
	case "3h":
		return time.Now().UTC().Add(-3 * time.Hour)
	case "6h":
		return time.Now().UTC().Add(-6 * time.Hour)
	case "24h":
		return time.Now().UTC().Add(-24 * time.Hour)
	case "7d":
		return time.Now().UTC().Add(-7 * 24 * time.Hour)
	case "30d":
		return time.Now().UTC().Add(-30 * 24 * time.Hour)
	default:
		return time.Now().UTC().Add(-fallback)
	}
}
