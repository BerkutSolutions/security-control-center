package handlers

import (
	"net/http"

	"berkut-scc/core/rbac"
)

func (h *MonitoringHandler) requirePerm(w http.ResponseWriter, r *http.Request, perm string) bool {
	if !hasPermission(r, h.policy, rbac.Permission(perm)) {
		http.Error(w, errForbidden, http.StatusForbidden)
		return false
	}
	return true
}
