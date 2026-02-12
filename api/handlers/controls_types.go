package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

func (h *ControlsHandler) ListControlTypes(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	items, err := h.store.ListControlTypes(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateControlType(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "settings.controls")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		http.Error(w, "controls.types.required", http.StatusBadRequest)
		return
	}
	item, err := h.store.CreateControlType(r.Context(), name, false)
	if err != nil {
		if err == store.ErrControlTypeExists {
			http.Error(w, "controls.types.exists", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.type.create", item.Name)
	writeJSON(w, http.StatusCreated, item)
}

func (h *ControlsHandler) DeleteControlType(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "settings.controls")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetControlTypeByID(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteControlType(r.Context(), id); err != nil {
		switch err {
		case store.ErrControlTypeBuiltin:
			http.Error(w, "controls.types.builtin", http.StatusBadRequest)
			return
		case store.ErrControlTypeInUse:
			http.Error(w, "controls.types.inUse", http.StatusConflict)
			return
		default:
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	h.logAudit(r.Context(), user.Username, "control.type.delete", item.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
