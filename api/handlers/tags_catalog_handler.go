package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

const tagsCatalogModuleID = "tags.catalog"

type TagsCatalogHandler struct {
	modules store.AppModuleStateStore
	audits  store.AuditStore
}

type tagCatalogItem struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func NewTagsCatalogHandler(modules store.AppModuleStateStore, audits store.AuditStore) *TagsCatalogHandler {
	return &TagsCatalogHandler{modules: modules, audits: audits}
}

func (h *TagsCatalogHandler) Get(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.load(r)})
}

func (h *TagsCatalogHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.modules == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var payload struct {
		Items []tagCatalogItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	items := normalizeTagCatalog(payload.Items)
	raw, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.modules.Upsert(r.Context(), &store.AppModuleState{
		ModuleID:               tagsCatalogModuleID,
		AppliedSchemaVersion:   1,
		AppliedBehaviorVersion: 1,
		LastError:              string(raw),
	}); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if h.audits != nil {
		_ = h.audits.Log(r.Context(), currentUsername(r), "tags.catalog.update", "items_count="+strconv.Itoa(len(items)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *TagsCatalogHandler) load(r *http.Request) []tagCatalogItem {
	if h == nil || h.modules == nil {
		return []tagCatalogItem{}
	}
	st, err := h.modules.Get(r.Context(), tagsCatalogModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return []tagCatalogItem{}
	}
	var payload struct {
		Items []tagCatalogItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		return []tagCatalogItem{}
	}
	return normalizeTagCatalog(payload.Items)
}

func normalizeTagCatalog(in []tagCatalogItem) []tagCatalogItem {
	out := make([]tagCatalogItem, 0, len(in))
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
		out = append(out, tagCatalogItem{Code: code, Label: label})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Label < out[j].Label
	})
	return out
}
