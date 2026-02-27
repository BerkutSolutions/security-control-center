package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func (h *MonitoringHandler) ListMonitorAssets(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if ok := h.requireAssetsView(w, r); !ok {
		return
	}
	mon, err := h.store.GetMonitor(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if mon == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	items, err := h.store.ListMonitorAssets(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MonitoringHandler) ReplaceMonitorAssets(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(pathParams(r)["id"])
	if err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	userID, ok := h.requireAssetsViewAndUserID(w, r)
	if !ok {
		return
	}
	mon, err := h.store.GetMonitor(r.Context(), id)
	if err != nil {
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	if mon == nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	var payload struct {
		AssetIDs []int64 `json:"asset_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, errBadRequest, http.StatusBadRequest)
		return
	}
	if len(payload.AssetIDs) > 500 {
		http.Error(w, "monitoring.assets.tooMany", http.StatusBadRequest)
		return
	}
	if err := h.store.ReplaceMonitorAssets(r.Context(), id, payload.AssetIDs, userID); err != nil {
		if strings.TrimSpace(err.Error()) == "monitoring.assets.assetNotFound" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, errServerError, http.StatusInternalServerError)
		return
	}
	h.audit(r, monitorAuditMonitorAssetsSet, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *MonitoringHandler) requireAssetsView(w http.ResponseWriter, r *http.Request) bool {
	_, ok := h.requireAssetsViewAndUserID(w, r)
	return ok
}

func (h *MonitoringHandler) requireAssetsViewAndUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	sess, err := sessionFromRequest(r)
	if err != nil || sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return 0, false
	}
	if h.policy == nil || !h.policy.Allowed(sess.Roles, "assets.view") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	if h.users == nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	user, _, err := h.users.Get(r.Context(), sess.UserID)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return 0, false
	}
	groups, _ := h.users.UserGroups(r.Context(), user.ID)
	eff := auth.CalculateEffectiveAccess(user, sess.Roles, groups, h.policy)
	if !allowedByMenuPermissions(eff.MenuPermissions, "assets") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return 0, false
	}
	return sess.UserID, true
}

func sessionFromRequest(r *http.Request) (*store.SessionRecord, error) {
	if r == nil {
		return nil, errors.New("no request")
	}
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		return nil, errors.New("no session")
	}
	sess, ok := val.(*store.SessionRecord)
	if !ok {
		return nil, errors.New("bad session")
	}
	return sess, nil
}
