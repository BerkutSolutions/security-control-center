package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

const servicesCatalogModuleID = "services.catalog"

type ServicesCatalogHandler struct {
	modules store.AppModuleStateStore
	audits  store.AuditStore
}

type serviceCatalogItem struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func NewServicesCatalogHandler(modules store.AppModuleStateStore, audits store.AuditStore) *ServicesCatalogHandler {
	return &ServicesCatalogHandler{modules: modules, audits: audits}
}

func (h *ServicesCatalogHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.load(r)})
}

func (h *ServicesCatalogHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload struct {
		Items []serviceCatalogItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	items := normalizeServiceCatalog(payload.Items)
	raw, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               servicesCatalogModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	}); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		_ = h.audits.Log(r.Context(), currentUsername(r), "services.catalog.update", "items_count="+strconv.Itoa(len(items)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ServicesCatalogHandler) load(r *http.Request) []serviceCatalogItem {
	if h == nil || h.modules == nil {
		return []serviceCatalogItem{}
	}
	st, err := h.modules.Get(r.Context(), servicesCatalogModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return []serviceCatalogItem{}
	}
	var payload struct {
		Items []serviceCatalogItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		return []serviceCatalogItem{}
	}
	return normalizeServiceCatalog(payload.Items)
}

func normalizeServiceCatalog(in []serviceCatalogItem) []serviceCatalogItem {
	out := make([]serviceCatalogItem, 0, len(in))
	seen := map[string]struct{}{}
	for _, item := range in {
		code := strings.ToUpper(strings.TrimSpace(item.Code))
		label := strings.TrimSpace(item.Label)
		if code == "" {
			code = strings.ToUpper(strings.ReplaceAll(label, " ", "_"))
		}
		if code == "" || label == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, serviceCatalogItem{Code: code, Label: label})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})
	return out
}
