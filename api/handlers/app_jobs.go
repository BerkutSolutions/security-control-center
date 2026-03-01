package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"github.com/go-chi/chi/v5"
)

type AppJobsHandler struct {
	jobs store.AppJobsStore
	policy *rbac.Policy
}

func NewAppJobsHandler(jobs store.AppJobsStore, policy *rbac.Policy) *AppJobsHandler {
	return &AppJobsHandler{jobs: jobs, policy: policy}
}

func (h *AppJobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.jobs == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	username := currentUsername(r)
	if username == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !hasPermission(r, h.policy, rbac.Permission("app.compat.manage.partial")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Type     string `json:"type"`
		Scope    string `json:"scope"`     // "module" | "all"
		ModuleID string `json:"module_id"` // required for scope=module
		Mode     string `json:"mode"`      // "full" | "partial"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	jobType := strings.ToLower(strings.TrimSpace(payload.Type))
	if jobType != "reinit" && jobType != "adapt" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	scope := strings.ToLower(strings.TrimSpace(payload.Scope))
	if scope == "" {
		scope = "module"
	}
	if scope != "module" && scope != "all" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode == "" {
		mode = "partial"
	}
	if mode != "full" && mode != "partial" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	moduleID := strings.TrimSpace(payload.ModuleID)
	if scope == "module" && moduleID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if scope == "all" {
		moduleID = ""
	}

	// Defense-in-depth: require module-level permission for destructive operations.
	if !h.allowedToStart(r, jobType, scope, moduleID, mode) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	id, err := h.jobs.Create(r.Context(), store.AppJobCreate{
		Type:      jobType,
		Scope:     scope,
		ModuleID:  moduleID,
		Mode:      mode,
		StartedBy: username,
	})
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *AppJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.jobs == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !hasPermission(r, h.policy, rbac.Permission("app.compat.view")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	items, err := h.jobs.ListRecent(r.Context(), limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AppJobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.jobs == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !hasPermission(r, h.policy, rbac.Permission("app.compat.view")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	raw := chi.URLParam(r, "id")
	id, err := parseID(raw)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	job, err := h.jobs.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if job == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (h *AppJobsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.jobs == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !hasPermission(r, h.policy, rbac.Permission("app.compat.manage.partial")) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	raw := chi.URLParam(r, "id")
	id, err := parseID(raw)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ok, err := h.jobs.Cancel(r.Context(), id, time.Now().UTC())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok})
}

func (h *AppJobsHandler) allowedToStart(r *http.Request, jobType, scope, moduleID, mode string) bool {
	// Global requirement: advanced settings for any reset-like action.
	if !hasPermission(r, h.policy, rbac.Permission("settings.advanced")) {
		return false
	}
	isFull := strings.ToLower(strings.TrimSpace(mode)) == "full"
	if isFull && !hasPermission(r, h.policy, rbac.Permission("app.compat.manage.full")) {
		return false
	}
	// Extra protection: critical modules full reset requires superadmin role (not just permissions).
	if strings.ToLower(strings.TrimSpace(scope)) == "all" {
		if isFull {
			// Full reset for "all" is always critical.
			return isSuperadmin(r)
		}
		return true
	}
	mod := strings.TrimSpace(moduleID)
	if mod == "" {
		return false
	}
	if isFull && isCriticalModule(mod) && !isSuperadmin(r) {
		return false
	}
	perm := requiredModulePerm(mod)
	if perm == "" {
		return true
	}
	return hasPermission(r, h.policy, perm)
}

func requiredModulePerm(moduleID string) rbac.Permission {
	switch strings.TrimSpace(moduleID) {
	case "tasks":
		return "tasks.manage"
	case "monitoring":
		return "monitoring.manage"
	case "docs":
		return "docs.manage"
	case "approvals":
		return "docs.manage"
	case "incidents":
		return "incidents.manage"
	case "registry.controls":
		return "controls.manage"
	case "registry.assets":
		return "assets.manage"
	case "registry.software":
		return "software.manage"
	case "registry.findings":
		return "findings.manage"
	case "reports":
		return "reports.edit"
	case "accounts":
		return "accounts.manage"
	case "backups":
		return "backups.restore"
	case "settings":
		return "settings.advanced"
	case "logs":
		return "logs.view"
	default:
		return ""
	}
}

func isCriticalModule(moduleID string) bool {
	switch strings.TrimSpace(moduleID) {
	case "docs", "incidents", "accounts", "logs":
		return true
	default:
		return false
	}
}

func isSuperadmin(r *http.Request) bool {
	sessVal := r.Context().Value(auth.SessionContextKey)
	if sessVal == nil {
		return false
	}
	sess, ok := sessVal.(*store.SessionRecord)
	if !ok || sess == nil {
		return false
	}
	for _, role := range sess.Roles {
		if strings.TrimSpace(strings.ToLower(role)) == "superadmin" {
			return true
		}
	}
	return false
}
