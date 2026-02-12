package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/core/docs"
	"berkut-scc/core/reports/charts"
	"berkut-scc/core/store"
)

type reportCreatePayload struct {
	Title               string   `json:"title"`
	ClassificationLevel string   `json:"classification_level"`
	ClassificationTags  []string `json:"classification_tags"`
	PeriodFrom          string   `json:"period_from"`
	PeriodTo            string   `json:"period_to"`
	Status              string   `json:"status"`
	Owner               *int64   `json:"owner"`
	ACLRoles            []string `json:"acl_roles"`
	ACLUsers            []int64  `json:"acl_users"`
	InheritACL          bool     `json:"inherit_acl"`
	TemplateID          *int64   `json:"template_id"`
}

type reportUpdatePayload struct {
	Title               string   `json:"title"`
	Status              string   `json:"status"`
	PeriodFrom          string   `json:"period_from"`
	PeriodTo            string   `json:"period_to"`
	ClassificationLevel string   `json:"classification_level"`
	ClassificationTags  []string `json:"classification_tags"`
	TemplateID          *int64   `json:"template_id"`
}

func (h *ReportsHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload reportCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Title) == "" {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	settings, _ := h.reports.GetReportSettings(r.Context())
	if payload.ClassificationLevel == "" && settings != nil {
		payload.ClassificationLevel = settings.DefaultClassification
	}
	doc, meta, content, err := h.buildReportFromPayload(r, user, roles, payload, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.saveReport(r, user, doc, meta, content, payload); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.create", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "meta": meta})
}

func (h *ReportsHandler) CreateFromTemplate(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload reportCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.TemplateID == nil || *payload.TemplateID == 0 {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	settings, _ := h.reports.GetReportSettings(r.Context())
	if payload.ClassificationLevel == "" && settings != nil {
		payload.ClassificationLevel = settings.DefaultClassification
	}
	tpl, err := h.reports.GetReportTemplate(r.Context(), *payload.TemplateID)
	if err != nil || tpl == nil {
		http.Error(w, localized(preferredLang(r), "reports.error.templateNotFound"), http.StatusNotFound)
		return
	}
	doc, meta, content, err := h.buildReportFromPayload(r, user, roles, payload, tpl.TemplateMarkdown)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	meta.TemplateID = payload.TemplateID
	if err := h.saveReport(r, user, doc, meta, content, payload); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.template.use", fmt.Sprintf("%d", *payload.TemplateID))
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "meta": meta})
}

func (h *ReportsHandler) Get(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, roles, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	h.log(r.Context(), user.Username, "report.view", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"doc":  doc,
		"meta": meta,
		"permissions": map[string]bool{
			"can_edit":   h.svc.CheckACL(user, roles, doc, nil, nil, "edit"),
			"can_export": h.svc.CheckACL(user, roles, doc, nil, nil, "export"),
			"can_manage": h.svc.CheckACL(user, roles, doc, nil, nil, "manage"),
		},
	})
}

func (h *ReportsHandler) UpdateMeta(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, roles, ok := h.loadReportForAccess(w, r, "edit")
	if !ok {
		return
	}
	var payload reportUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Title) != "" {
		doc.Title = strings.TrimSpace(payload.Title)
	}
	if payload.Status != "" {
		status, ok := normalizeReportStatus(payload.Status)
		if !ok {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidStatus"), http.StatusBadRequest)
			return
		}
		meta.Status = status
	}
	if payload.TemplateID != nil {
		meta.TemplateID = payload.TemplateID
	}
	if payload.PeriodFrom != "" || payload.PeriodTo != "" {
		start, err := parseDateStrict(payload.PeriodFrom)
		if err != nil && payload.PeriodFrom != "" {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidPeriod"), http.StatusBadRequest)
			return
		}
		end, err := parseDateStrict(payload.PeriodTo)
		if err != nil && payload.PeriodTo != "" {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidPeriod"), http.StatusBadRequest)
			return
		}
		meta.PeriodFrom = start
		meta.PeriodTo = end
	}
	if payload.ClassificationLevel != "" {
		level, err := docs.ParseLevel(payload.ClassificationLevel)
		if err != nil {
			http.Error(w, localized(preferredLang(r), "reports.error.invalidClassification"), http.StatusBadRequest)
			return
		}
		if !hasPrivRole(roles) && !docs.HasClearance(docs.ClassificationLevel(user.ClearanceLevel), user.ClearanceTags, level, payload.ClassificationTags) {
			http.Error(w, localized(preferredLang(r), "reports.error.forbidden"), http.StatusForbidden)
			return
		}
		doc.ClassificationLevel = int(level)
		doc.ClassificationTags = docs.NormalizeTags(payload.ClassificationTags)
	}
	if err := h.docs.UpdateDocument(r.Context(), doc); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if err := h.reports.UpsertReportMeta(r.Context(), meta); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.update_meta", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "meta": meta})
}

func (h *ReportsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "manage")
	if !ok {
		return
	}
	if err := h.docs.SoftDeleteDocument(r.Context(), doc.ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.delete", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ReportsHandler) buildReportFromPayload(r *http.Request, user *store.User, roles []string, payload reportCreatePayload, templateMd string) (*store.Document, *store.ReportMeta, []byte, error) {
	level, err := docs.ParseLevel(payload.ClassificationLevel)
	if err != nil {
		return nil, nil, nil, errors.New(localized(preferredLang(r), "reports.error.invalidClassification"))
	}
	if !hasPrivRole(roles) && !docs.HasClearance(docs.ClassificationLevel(user.ClearanceLevel), user.ClearanceTags, level, payload.ClassificationTags) {
		return nil, nil, nil, errors.New(localized(preferredLang(r), "reports.error.forbidden"))
	}
	start, err := parseDateStrict(payload.PeriodFrom)
	if err != nil && payload.PeriodFrom != "" {
		return nil, nil, nil, errors.New(localized(preferredLang(r), "reports.error.invalidPeriod"))
	}
	end, err := parseDateStrict(payload.PeriodTo)
	if err != nil && payload.PeriodTo != "" {
		return nil, nil, nil, errors.New(localized(preferredLang(r), "reports.error.invalidPeriod"))
	}
	createdBy := user.ID
	if payload.Owner != nil && *payload.Owner > 0 && hasPrivRole(roles) {
		createdBy = *payload.Owner
	}
	doc := &store.Document{
		Title:                 strings.TrimSpace(payload.Title),
		Status:                docs.StatusDraft,
		ClassificationLevel:   int(level),
		ClassificationTags:    docs.NormalizeTags(payload.ClassificationTags),
		DocType:               "report",
		InheritACL:            payload.InheritACL,
		InheritClassification: true,
		CreatedBy:             createdBy,
		CurrentVersion:        0,
	}
	meta := &store.ReportMeta{
		PeriodFrom: start,
		PeriodTo:   end,
		Status:     strings.TrimSpace(payload.Status),
	}
	if status, ok := normalizeReportStatus(meta.Status); ok {
		meta.Status = status
	} else {
		return nil, nil, nil, errors.New(localized(preferredLang(r), "reports.error.invalidStatus"))
	}
	settings, _ := h.reports.GetReportSettings(r.Context())
	content := h.defaultReportMarkdown(doc.Title)
	if templateMd != "" {
		content = applyTemplate(templateMd, h.templateVars(doc.Title, meta, settings))
	}
	return doc, meta, []byte(content), nil
}

func (h *ReportsHandler) saveReport(r *http.Request, user *store.User, doc *store.Document, meta *store.ReportMeta, content []byte, payload reportCreatePayload) error {
	acl := buildBaseACLFor(user)
	if len(payload.ACLRoles) > 0 || len(payload.ACLUsers) > 0 {
		acl = append(acl, buildACLFor(payload.ACLRoles, payload.ACLUsers)...)
	}
	docID, err := h.docs.CreateDocument(r.Context(), doc, acl, h.cfg.Docs.RegTemplate, h.cfg.Docs.PerFolderSequence)
	if err != nil {
		return err
	}
	doc.ID = docID
	meta.DocID = docID
	if err := h.reports.UpsertReportMeta(r.Context(), meta); err != nil {
		return err
	}
	_ = h.reports.ReplaceReportCharts(r.Context(), docID, charts.DefaultCharts())
	_, err = h.svc.SaveVersion(r.Context(), docs.SaveRequest{
		Doc:      doc,
		Author:   user,
		Format:   docs.FormatMarkdown,
		Content:  content,
		Reason:   "initial",
		IndexFTS: true,
	})
	return err
}
