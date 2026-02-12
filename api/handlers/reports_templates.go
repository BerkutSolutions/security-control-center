package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

func (h *ReportsHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.reports.EnsureDefaultReportTemplates(r.Context()); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	templates, err := h.reports.ListReportTemplates(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.template.list", fmt.Sprintf("%d", len(templates)))
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func (h *ReportsHandler) SaveTemplate(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		TemplateMarkdown string `json:"template_markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Name) == "" {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	tpl := &store.ReportTemplate{
		ID:               payload.ID,
		Name:             payload.Name,
		Description:      payload.Description,
		TemplateMarkdown: payload.TemplateMarkdown,
		CreatedBy:        user.ID,
	}
	if tpl.ID != 0 {
		existing, _ := h.reports.GetReportTemplate(r.Context(), tpl.ID)
		if existing != nil {
			tpl.CreatedBy = existing.CreatedBy
			tpl.CreatedAt = existing.CreatedAt
		}
	}
	if err := h.reports.SaveReportTemplate(r.Context(), tpl); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	action := "report.template.create"
	if payload.ID != 0 {
		action = "report.template.update"
	}
	h.log(r.Context(), user.Username, action, tpl.Name)
	writeJSON(w, http.StatusOK, map[string]any{"template": tpl})
}

func (h *ReportsHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := h.reports.DeleteReportTemplate(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.template.delete", fmt.Sprintf("%d", id))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
