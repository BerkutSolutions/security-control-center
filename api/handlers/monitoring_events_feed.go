package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

type monitoringEventItem struct {
	ID          int64     `json:"id"`
	MonitorID   int64     `json:"monitor_id"`
	MonitorName string    `json:"monitor_name"`
	TS          time.Time `json:"ts"`
	EventType   string    `json:"event_type"`
	Message     string    `json:"message"`
}

func (h *MonitoringHandler) EventsFeed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.EventFilter{
		Since: rangeSince(q.Get("range"), 24*time.Hour),
		Types: splitCSV(q.Get("type")),
		Tags:  splitCSV(q.Get("tag")),
		Limit: 50,
	}
	if val := strings.TrimSpace(q.Get("monitor_id")); val != "" {
		if id, err := strconv.ParseInt(val, 10, 64); err == nil && id > 0 {
			filter.MonitorID = &id
		}
	}
	if val := strings.TrimSpace(q.Get("limit")); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	items, err := h.store.ListEventsFeed(r.Context(), filter)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	monitors, _ := h.store.ListMonitors(r.Context(), store.MonitorFilter{})
	names := map[int64]string{}
	for _, m := range monitors {
		names[m.ID] = m.Name
	}
	var out []monitoringEventItem
	for _, item := range items {
		out = append(out, monitoringEventItem{
			ID:          item.ID,
			MonitorID:   item.MonitorID,
			MonitorName: names[item.MonitorID],
			TS:          item.TS,
			EventType:   item.EventType,
			Message:     item.Message,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out,
		"from":  filter.Since,
		"to":    time.Now().UTC(),
	})
}
