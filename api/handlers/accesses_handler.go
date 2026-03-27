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
		Items []map[string]any `json:"items"`
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
		Items []map[string]any `json:"items"`
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
		_ = h.audits.Log(r.Context(), currentUsername(r), "accesses.update", "items_count="+strconv.Itoa(len(payload.Items)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
