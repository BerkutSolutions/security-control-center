package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"berkut-scc/core/docs"
	"berkut-scc/core/reports/charts"
	"berkut-scc/core/store"
	"berkut-scc/gui"
)

func (h *ReportsHandler) Export(w http.ResponseWriter, r *http.Request) {
	doc, meta, user, _, ok := h.loadReportForAccess(w, r, "export")
	if !ok {
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if format == "" {
		format = docs.FormatMarkdown
	}
	normalizedFormat := strings.ToLower(strings.TrimPrefix(format, "."))
	ver, err := h.docs.GetVersion(r.Context(), doc.ID, doc.CurrentVersion)
	if err != nil || ver == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	content, err := h.svc.LoadContent(r.Context(), ver)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	settings, _ := h.reports.GetReportSettings(r.Context())
	header, _ := h.buildReportHeader(settings, doc, meta)
	md := append(header, content...)
	md = append(md, '\n', '\n')
	chartList, _ := h.reports.ListReportCharts(r.Context(), doc.ID)
	chartList = filterEnabledCharts(chartList)
	cleanup := func() {}
	if len(chartList) > 0 {
		if (normalizedFormat == docs.FormatDocx || normalizedFormat == docs.FormatPDF) && !h.svc.ConvertersStatus().PandocAvailable {
			http.Error(w, localized(preferredLang(r), "reports.error.exportChartsUnavailable"), http.StatusBadRequest)
			return
		}
		snaps, _ := h.reports.ListReportSnapshots(r.Context(), doc.ID)
		if len(snaps) == 0 {
			http.Error(w, localized(preferredLang(r), "reports.error.snapshotRequired"), http.StatusBadRequest)
			return
		}
		snapshot, items, err := h.reports.GetReportSnapshot(r.Context(), snaps[0].ID)
		if err != nil || snapshot == nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		var errAppend error
		md, cleanup, errAppend = h.appendChartsToMarkdown(r.Context(), md, chartList, snapshot, items, preferredLang(r), normalizedFormat)
		if errAppend != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		defer cleanup()
	}
	wm, _ := h.svc.WatermarkFor(doc, user.Username)
	data, ct, err := h.svc.ConvertMarkdown(r.Context(), format, md, wm)
	if err != nil {
		http.Error(w, localized(preferredLang(r), "reports.error.exportConverterMissing"), http.StatusBadRequest)
		return
	}
	filename := fmt.Sprintf("%s.%s", safeFileName(doc.RegNumber), strings.ToLower(strings.TrimPrefix(format, ".")))
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	h.log(r.Context(), user.Username, "report.export", doc.RegNumber)
	if len(chartList) > 0 {
		h.log(r.Context(), user.Username, "report.export.with_charts", doc.RegNumber)
	}
}

func filterEnabledCharts(items []store.ReportChart) []store.ReportChart {
	var out []store.ReportChart
	for _, ch := range items {
		if ch.IsEnabled {
			out = append(out, ch)
		}
	}
	return out
}

func (h *ReportsHandler) appendChartsToMarkdown(ctx context.Context, base []byte, list []store.ReportChart, snapshot *store.ReportSnapshot, items []store.ReportSnapshotItem, lang, format string) ([]byte, func(), error) {
	if len(items) > 5000 {
		items = items[:5000]
	}
	useTemp := format == docs.FormatDocx || format == docs.FormatPDF
	tempDir := ""
	if useTemp {
		dir, err := os.MkdirTemp("", "report-charts-*")
		if err != nil {
			return base, func() {}, err
		}
		tempDir = dir
	}
	var b strings.Builder
	b.WriteString("\n\n## ")
	b.WriteString(chartsSectionTitle(lang))
	b.WriteString("\n\n")
	for i, ch := range list {
		def, ok := charts.DefinitionFor(ch.ChartType)
		if !ok {
			continue
		}
		ch.Config = charts.NormalizeConfig(ch.ChartType, ch.Config)
		ch.Title = charts.Localized(lang, def.TitleKey)
		data, err := charts.BuildChart(ch, snapshot, items, lang)
		if err != nil {
			continue
		}
		svg, err := charts.RenderSVG(data)
		if err != nil {
			continue
		}
		label := ch.Title
		path := ""
		if useTemp {
			filename := fmt.Sprintf("chart_%d.svg", i+1)
			path = filepath.Join(tempDir, filename)
			if err := os.WriteFile(path, svg, 0o600); err != nil {
				continue
			}
		} else {
			path = "data:image/svg+xml;utf8," + url.QueryEscape(string(svg))
		}
		b.WriteString(fmt.Sprintf("![%s](%s)\n\n", label, path))
	}
	cleanup := func() {}
	if tempDir != "" {
		cleanup = func() {
			_ = os.RemoveAll(tempDir)
		}
	}
	md := append(base, []byte(b.String())...)
	return md, cleanup, nil
}

func chartsSectionTitle(lang string) string {
	if lang == "ru" {
		return "Графики"
	}
	return "Charts"
}

func (h *ReportsHandler) buildReportHeader(settings *store.ReportSettings, doc *store.Document, meta *store.ReportMeta) ([]byte, error) {
	if settings == nil || !settings.HeaderEnabled {
		return nil, nil
	}
	var buf bytes.Buffer
	logo := h.logoDataURI(settings.HeaderLogoPath)
	hasTop := logo != "" || strings.TrimSpace(settings.HeaderTitle) != ""
	if hasTop {
		buf.WriteString("<div style=\"display:flex;align-items:center;gap:12px;\">")
		if logo != "" {
			fmt.Fprintf(&buf, "<img src=\"%s\" style=\"height:48px;\"/>", logo)
		}
		if strings.TrimSpace(settings.HeaderTitle) != "" {
			fmt.Fprintf(&buf, "<div style=\"font-size:16px;font-weight:600;\">%s</div>", html.EscapeString(strings.TrimSpace(settings.HeaderTitle)))
		}
		buf.WriteString("</div>\n")
	}
	if doc != nil && strings.TrimSpace(doc.Title) != "" {
		fmt.Fprintf(&buf, "<div style=\"margin-top:8px;font-size:18px;font-weight:600;\">%s</div>\n", html.EscapeString(strings.TrimSpace(doc.Title)))
	}
	if meta != nil {
		if period := formatPeriod(meta.PeriodFrom, meta.PeriodTo); period != "" {
			fmt.Fprintf(&buf, "<div style=\"margin-top:4px;\">%s</div>\n", html.EscapeString(period))
		}
	}
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func (h *ReportsHandler) logoDataURI(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	rel := strings.TrimPrefix(trimmed, "/")
	if strings.HasPrefix(rel, "gui/") {
		rel = strings.TrimPrefix(rel, "gui/")
	}
	if !strings.HasPrefix(rel, "static/") {
		rel = filepath.Join("static", rel)
	}
	data, err := gui.StaticFiles.ReadFile(rel)
	if err != nil {
		if h.logger != nil {
			h.logger.Errorf("report logo load failed: %v", err)
		}
		return ""
	}
	return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(data))
}

func (h *ReportsHandler) templateVars(title string, meta *store.ReportMeta, settings *store.ReportSettings) map[string]string {
	out := map[string]string{
		"report_title": title,
	}
	if meta != nil {
		if meta.PeriodFrom != nil {
			out["period_from"] = meta.PeriodFrom.Format("2006-01-02")
		}
		if meta.PeriodTo != nil {
			out["period_to"] = meta.PeriodTo.Format("2006-01-02")
		}
	}
	if settings != nil && settings.HeaderTitle != "" {
		out["org_name"] = settings.HeaderTitle
	}
	return out
}
