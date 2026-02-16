package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"berkut-scc/core/store"
)

var reportSectionTypes = map[string]struct{}{
	"summary":     {},
	"incidents":   {},
	"tasks":       {},
	"docs":        {},
	"controls":    {},
	"monitoring":  {},
	"sla_summary": {},
	"audit":       {},
	"custom_md":   {},
}

func defaultReportSections() []store.ReportSection {
	return []store.ReportSection{
		{SectionType: "summary", Title: "Executive summary", IsEnabled: true},
		{SectionType: "incidents", Title: "Incidents", IsEnabled: true},
		{SectionType: "tasks", Title: "Tasks", IsEnabled: true},
		{SectionType: "docs", Title: "Documents", IsEnabled: true},
		{SectionType: "controls", Title: "Controls", IsEnabled: true},
		{SectionType: "monitoring", Title: "Monitoring", IsEnabled: true},
		{SectionType: "sla_summary", Title: "SLA executive summary", IsEnabled: true},
		{SectionType: "audit", Title: "Audit events", IsEnabled: true},
	}
}

func (h *ReportsHandler) ListSections(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	sections, err := h.reports.ListReportSections(r.Context(), doc.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if len(sections) == 0 {
		sections = defaultReportSections()
	}
	h.log(r.Context(), user.Username, "report.sections.view", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"sections": sections,
		"meta":     meta,
	})
}

func (h *ReportsHandler) UpdateSections(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, _, ok := h.loadReportForAccess(w, r, "edit")
	if !ok {
		return
	}
	var payload struct {
		PeriodFrom string                `json:"period_from"`
		PeriodTo   string                `json:"period_to"`
		Sections   []store.ReportSection `json:"sections"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	for i := range payload.Sections {
		payload.Sections[i].SectionType = strings.TrimSpace(strings.ToLower(payload.Sections[i].SectionType))
		if _, ok := reportSectionTypes[payload.Sections[i].SectionType]; !ok {
			http.Error(w, localized(preferredLang(r), "reports.error.sectionTypeInvalid"), http.StatusBadRequest)
			return
		}
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
		if meta == nil {
			meta = &store.ReportMeta{DocID: doc.ID, Status: "draft"}
		}
		meta.PeriodFrom = start
		meta.PeriodTo = end
		if err := h.reports.UpsertReportMeta(r.Context(), meta); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	if err := h.reports.ReplaceReportSections(r.Context(), doc.ID, payload.Sections); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.sections.update", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
