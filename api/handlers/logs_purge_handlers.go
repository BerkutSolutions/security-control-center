package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type logsPurgeCreatePayload struct {
	RetentionDays int    `json:"retention_days"`
	Reason        string `json:"reason"`
}

func (h *LogsHandler) ListPurgeRequests(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.audits == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	items, err := h.audits.ListPurgeRequests(r.Context(), limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *LogsHandler) CreatePurgeRequest(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.audits == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload logsPurgeCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	payload.Reason = strings.TrimSpace(payload.Reason)
	if payload.RetentionDays < 1 || payload.RetentionDays > 3650 {
		http.Error(w, "invalid retention_days", http.StatusBadRequest)
		return
	}
	item, err := h.audits.CreatePurgeRequest(r.Context(), currentUsername(r), payload.RetentionDays, payload.Reason)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (h *LogsHandler) ApprovePurgeRequest(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.audits == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	idStr := strings.TrimSpace(chi.URLParam(r, "id"))
	if idStr == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	result, err := h.audits.ApproveAndExecutePurge(r.Context(), id, currentUsername(r))
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "second approver"):
			http.Error(w, "dual control violation", http.StatusConflict)
		case strings.Contains(err.Error(), "expired"), strings.Contains(err.Error(), "not pending"):
			http.Error(w, "request unavailable", http.StatusConflict)
		case strings.Contains(err.Error(), "conflict"):
			http.Error(w, "request conflict", http.StatusConflict)
		default:
			http.Error(w, "server error", http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}
