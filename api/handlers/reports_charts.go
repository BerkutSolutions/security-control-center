package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/reports/charts"
	"berkut-scc/core/store"
	"github.com/gorilla/mux"
)

func (h *ReportsHandler) ListCharts(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	items, err := h.reports.ListReportCharts(r.Context(), doc.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if len(items) == 0 {
		defaults := charts.DefaultCharts()
		_ = h.reports.ReplaceReportCharts(r.Context(), doc.ID, defaults)
		items, _ = h.reports.ListReportCharts(r.Context(), doc.ID)
	}
	lang := preferredLang(r)
	normalized := make([]store.ReportChart, 0, len(items))
	for _, ch := range items {
		if def, ok := charts.DefinitionFor(ch.ChartType); ok {
			ch.Config = charts.NormalizeConfig(ch.ChartType, ch.Config)
			ch.Title = charts.Localized(lang, def.TitleKey)
			if strings.TrimSpace(ch.SectionType) == "" {
				ch.SectionType = def.SectionType
			}
		}
		normalized = append(normalized, ch)
	}
	snapshots, _ := h.reports.ListReportSnapshots(r.Context(), doc.ID)
	snapshotID := int64(0)
	if len(snapshots) > 0 {
		snapshotID = snapshots[0].ID
	}
	h.log(r.Context(), user.Username, "report.charts.list", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{
		"charts":      normalized,
		"snapshot_id": snapshotID,
	})
}

func (h *ReportsHandler) UpdateCharts(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "edit")
	if !ok {
		return
	}
	var payload struct {
		Charts []store.ReportChart `json:"charts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	if len(payload.Charts) > 30 {
		payload.Charts = payload.Charts[:30]
	}
	lang := preferredLang(r)
	var normalized []store.ReportChart
	for _, ch := range payload.Charts {
		def, ok := charts.DefinitionFor(ch.ChartType)
		if !ok {
			http.Error(w, localized(lang, "reports.error.chartNotFound"), http.StatusBadRequest)
			return
		}
		ch.Config = charts.NormalizeConfig(ch.ChartType, ch.Config)
		if strings.TrimSpace(ch.Title) == "" || strings.HasPrefix(ch.Title, "chart.") {
			ch.Title = charts.Localized(lang, def.TitleKey)
		}
		ch.SectionType = def.SectionType
		normalized = append(normalized, ch)
	}
	if err := h.reports.ReplaceReportCharts(r.Context(), doc.ID, normalized); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.log(r.Context(), user.Username, "report.charts.update", doc.RegNumber)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *ReportsHandler) RenderChart(w http.ResponseWriter, r *http.Request) {
	doc, _, user, _, ok := h.loadReportForAccess(w, r, "view")
	if !ok {
		return
	}
	idStr := mux.Vars(r)["chart_id"]
	chartID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || chartID == 0 {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	ch, err := h.reports.GetReportChart(r.Context(), chartID)
	if err != nil || ch == nil || ch.ReportID != doc.ID || !ch.IsEnabled {
		http.Error(w, localized(preferredLang(r), "reports.error.chartNotFound"), http.StatusNotFound)
		return
	}
	snapshots, _ := h.reports.ListReportSnapshots(r.Context(), doc.ID)
	if len(snapshots) == 0 {
		http.Error(w, localized(preferredLang(r), "reports.error.snapshotRequired"), http.StatusBadRequest)
		return
	}
	snapshot, items, err := h.reports.GetReportSnapshot(r.Context(), snapshots[0].ID)
	if err != nil || snapshot == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if len(items) > 5000 {
		items = items[:5000]
	}
	chartData, err := charts.BuildChart(*ch, snapshot, items, preferredLang(r))
	if err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.badRequest"), http.StatusBadRequest)
		return
	}
	svg, err := charts.RenderSVG(chartData)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(svg)
	h.log(r.Context(), user.Username, "report.charts.render", doc.RegNumber)
}
