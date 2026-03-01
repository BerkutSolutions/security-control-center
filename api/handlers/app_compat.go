package handlers

import (
	"net/http"
	"time"

	"berkut-scc/core/appcompat"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type AppCompatHandler struct {
	svc *appcompat.Service
	policy *rbac.Policy
}

func NewAppCompatHandler(appModules store.AppModuleStateStore, policy *rbac.Policy) *AppCompatHandler {
	return &AppCompatHandler{
		svc:    appcompat.NewService(appModules, appcompat.DefaultRegistry()),
		policy: policy,
	}
}

func (h *AppCompatHandler) Report(w http.ResponseWriter, r *http.Request) {
	if !hasPermission(r, h.policy, rbac.Permission("app.compat.view")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	report, err := h.svc.Report(r.Context(), time.Now().UTC())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
