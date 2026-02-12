package handlers

import (
	"net/http"

	"berkut-scc/core/store"
)

type LogsHandler struct {
	audits store.AuditStore
}

func NewLogsHandler(audits store.AuditStore) *LogsHandler {
	return &LogsHandler{audits: audits}
}

func (h *LogsHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.audits.List(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
