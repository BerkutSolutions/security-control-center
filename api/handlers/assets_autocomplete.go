package handlers

import (
	"net/http"
	"strings"
)

func (h *AssetsHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "assets.manage")
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
		owners, _ := h.store.SuggestAssetOwners(r.Context(), q, limit, includeDeleted)
		admins, _ := h.store.SuggestAssetAdministrators(r.Context(), q, limit, includeDeleted)
		tags, _ := h.store.SuggestAssetTags(r.Context(), q, limit, includeDeleted)
		resp["owners"] = owners
		resp["administrators"] = admins
		resp["tags"] = tags
	case "owner", "owners":
		owners, _ := h.store.SuggestAssetOwners(r.Context(), q, limit, includeDeleted)
		resp["owners"] = owners
	case "administrator", "administrators", "admin":
		admins, _ := h.store.SuggestAssetAdministrators(r.Context(), q, limit, includeDeleted)
		resp["administrators"] = admins
	case "tag", "tags":
		tags, _ := h.store.SuggestAssetTags(r.Context(), q, limit, includeDeleted)
		resp["tags"] = tags
	default:
		http.Error(w, "assets.autocomplete.fieldInvalid", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
