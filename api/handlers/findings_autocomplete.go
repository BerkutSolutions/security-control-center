package handlers

import (
	"net/http"
	"strings"
)

func (h *FindingsHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.view")
	if !ok {
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(sess.Roles, "findings.manage")
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
		titles, _ := h.store.SuggestFindingTitles(r.Context(), q, limit, includeDeleted)
		owners, _ := h.store.SuggestFindingOwners(r.Context(), q, limit, includeDeleted)
		tags, _ := h.store.SuggestFindingTags(r.Context(), q, limit, includeDeleted)
		resp["titles"] = titles
		resp["owners"] = owners
		resp["tags"] = tags
	case "title", "titles":
		titles, _ := h.store.SuggestFindingTitles(r.Context(), q, limit, includeDeleted)
		resp["titles"] = titles
	case "owner", "owners":
		owners, _ := h.store.SuggestFindingOwners(r.Context(), q, limit, includeDeleted)
		resp["owners"] = owners
	case "tag", "tags":
		tags, _ := h.store.SuggestFindingTags(r.Context(), q, limit, includeDeleted)
		resp["tags"] = tags
	default:
		http.Error(w, "findings.autocomplete.fieldInvalid", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
