package taskshttp

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetBoardLayout(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	spaceID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if spaceID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	space, err := h.svc.Store().GetSpace(r.Context(), spaceID)
	if err != nil || space == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	acl, _ := h.svc.Store().GetSpaceACL(r.Context(), space.ID)
	if !aclAllowed(user, roles, groups, acl, "view") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	layout, err := h.svc.Store().GetBoardLayout(r.Context(), user.ID, spaceID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	var raw json.RawMessage
	if strings.TrimSpace(layout) != "" {
		raw = json.RawMessage(layout)
	}
	respondJSON(w, http.StatusOK, map[string]any{"layout": raw})
}

func (h *Handler) SaveBoardLayout(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	spaceID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if spaceID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	space, err := h.svc.Store().GetSpace(r.Context(), spaceID)
	if err != nil || space == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	acl, _ := h.svc.Store().GetSpaceACL(r.Context(), space.ID)
	if !aclAllowed(user, roles, groups, acl, "view") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Layout json.RawMessage `json:"layout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	layout := strings.TrimSpace(string(payload.Layout))
	if layout == "" {
		layout = "{}"
	}
	if err := h.svc.Store().SaveBoardLayout(r.Context(), user.ID, spaceID, layout); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
