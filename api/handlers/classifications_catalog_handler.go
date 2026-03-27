package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

const classificationsCatalogModuleID = "classifications.catalog"

type ClassificationsCatalogHandler struct {
	modules store.AppModuleStateStore
	audits  store.AuditStore
}

type classificationsCatalogPayload struct {
	Labels map[string]string `json:"labels"`
	Order  []string          `json:"order"`
}

func NewClassificationsCatalogHandler(modules store.AppModuleStateStore, audits store.AuditStore) *ClassificationsCatalogHandler {
	return &ClassificationsCatalogHandler{modules: modules, audits: audits}
}

func (h *ClassificationsCatalogHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.load(r))
}

func (h *ClassificationsCatalogHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload classificationsCatalogPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	payload = normalizeClassificationsCatalog(payload)
	raw, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               classificationsCatalogModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	}); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		_ = h.audits.Log(r.Context(), currentUsername(r), "classifications.catalog.update", "order_count="+strconv.Itoa(len(payload.Order)))
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *ClassificationsCatalogHandler) load(r *http.Request) classificationsCatalogPayload {
	defaults := classificationsCatalogPayload{
		Labels: map[string]string{},
		Order:  []string{},
	}
	if h == nil || h.modules == nil {
		return defaults
	}
	st, err := h.modules.Get(r.Context(), classificationsCatalogModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return defaults
	}
	var payload classificationsCatalogPayload
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		return defaults
	}
	return normalizeClassificationsCatalog(payload)
}

func normalizeClassificationsCatalog(in classificationsCatalogPayload) classificationsCatalogPayload {
	out := classificationsCatalogPayload{
		Labels: map[string]string{},
		Order:  make([]string, 0, len(in.Order)),
	}
	for k, v := range in.Labels {
		key := strings.ToUpper(strings.TrimSpace(k))
		val := strings.TrimSpace(v)
		if key == "" || val == "" {
			continue
		}
		out.Labels[key] = val
	}
	seen := map[string]struct{}{}
	for _, item := range in.Order {
		code := strings.ToUpper(strings.TrimSpace(item))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		out.Order = append(out.Order, code)
	}
	return out
}
