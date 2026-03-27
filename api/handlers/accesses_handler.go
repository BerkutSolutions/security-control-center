package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

const accessesModuleID = "accesses.data"

type AccessesHandler struct {
	modules store.AppModuleStateStore
	audits  store.AuditStore
}

type accessesAuditEvent struct {
	Type     string   `json:"type"`
	User     string   `json:"user"`
	Services []string `json:"services"`
	Details  string   `json:"details"`
}

func NewAccessesHandler(modules store.AppModuleStateStore, audits store.AuditStore) *AccessesHandler {
	return &AccessesHandler{modules: modules, audits: audits}
}

func (h *AccessesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	st, err := h.modules.Get(r.Context(), accessesModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	var payload struct {
		Items []map[string]any    `json:"items"`
		Event *accessesAuditEvent `json:"event,omitempty"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": payload.Items})
}

func (h *AccessesHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload struct {
		Items []map[string]any    `json:"items"`
		Event *accessesAuditEvent `json:"event,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	raw, err := json.Marshal(map[string]any{"items": payload.Items})
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err = h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               accessesModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		action, details := formatAccessesAudit(payload.Event, len(payload.Items))
		_ = h.audits.Log(r.Context(), currentUsername(r), action, details)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func formatAccessesAudit(event *accessesAuditEvent, itemsCount int) (string, string) {
	base := "items_count=" + strconv.Itoa(itemsCount)
	if event == nil {
		return "accesses.update", base
	}
	eventType := strings.ToLower(strings.TrimSpace(event.Type))
	action := map[string]string{
		"create":     "accesses.create",
		"edit":       "accesses.edit",
		"supplement": "accesses.supplement",
		"blocked":    "accesses.blocked",
		"unblocked":  "accesses.unblocked",
		"delete":     "accesses.delete",
		"dismissal":  "accesses.dismissal",
		"test":       "accesses.test",
		"cleanup":    "accesses.cleanup",
	}[eventType]
	if action == "" {
		action = "accesses.update"
	}
	user := strings.TrimSpace(event.User)
	if user != "" {
		base += " user=" + user
	}
	if n := len(event.Services); n > 0 {
		base += " services_count=" + strconv.Itoa(n)
	}
	details := strings.TrimSpace(event.Details)
	if details != "" {
		base += " details=" + details
	}
	return action, base
}
