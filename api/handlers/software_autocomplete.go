package handlers

import (
	"net/http"
	"strings"
)

func (h *SoftwareHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "software.manage")
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	field := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("field")))
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	includeDeleted := canManage && parseBool(r.URL.Query().Get("include_deleted"))

	resp := map[string]any{}
	switch field {
	case "", "all":
		names, _ := h.store.SuggestProductNames(r.Context(), q, limit, includeDeleted)
		vendors, _ := h.store.SuggestVendors(r.Context(), q, limit, includeDeleted)
		tags, _ := h.store.SuggestProductTags(r.Context(), q, limit, includeDeleted)
		resp["names"] = names
		resp["vendors"] = vendors
		resp["tags"] = tags
	case "name", "names":
		names, _ := h.store.SuggestProductNames(r.Context(), q, limit, includeDeleted)
		resp["names"] = names
	case "vendor", "vendors":
		vendors, _ := h.store.SuggestVendors(r.Context(), q, limit, includeDeleted)
		resp["vendors"] = vendors
	case "tag", "tags":
		tags, _ := h.store.SuggestProductTags(r.Context(), q, limit, includeDeleted)
		resp["tags"] = tags
	default:
		http.Error(w, "software.autocomplete.fieldInvalid", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
