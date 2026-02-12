package taskshttp

import (
	"net/http"

	"berkut-scc/tasks"
)

func (h *Handler) ListTags(w http.ResponseWriter, r *http.Request) {
	user, roles, _, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermView) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	items, err := h.svc.Store().ListTaskTags(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}
