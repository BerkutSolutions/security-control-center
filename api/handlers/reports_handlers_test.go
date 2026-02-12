package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
)

func TestReportHeaderInjection(t *testing.T) {
	h := &ReportsHandler{}
	settings := &store.ReportSettings{
		HeaderEnabled:  true,
		HeaderLogoPath: "/gui/static/logo.png",
		HeaderTitle:    "Berkut Solutions: Security Control Center",
	}
	doc := &store.Document{
		Title:     "Monthly Report",
		RegNumber: "R-001",
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	meta := &store.ReportMeta{
		PeriodFrom: &start,
		PeriodTo:   &end,
	}
	header, err := h.buildReportHeader(settings, doc, meta)
	if err != nil {
		t.Fatalf("header build: %v", err)
	}
	text := string(header)
	if !strings.Contains(text, "data:image/png;base64") {
		t.Fatalf("expected data uri logo in header")
	}
	if !strings.Contains(text, "Monthly Report") {
		t.Fatalf("expected report title in header")
	}
	if !strings.Contains(text, "2026-01-01") {
		t.Fatalf("expected period in header")
	}
}

func TestApplySectionContentMarkers(t *testing.T) {
	base := "Intro\n\n<!-- SECTION: incidents -->\nold\n<!-- ENDSECTION -->\n\nFooter"
	sections := []reportSectionResult{
		{Section: store.ReportSection{SectionType: "incidents", IsEnabled: true}, Markdown: "## Incidents\n\nNew"},
	}
	out := applySectionContent("markers", base, sections)
	if !strings.Contains(out, "Intro") || !strings.Contains(out, "Footer") {
		t.Fatalf("expected to preserve manual content")
	}
	if !strings.Contains(out, "## Incidents") || strings.Contains(out, "old") {
		t.Fatalf("expected to replace section content")
	}
}

func TestAppendChartsToMarkdownEmbedsImages(t *testing.T) {
	h := &ReportsHandler{}
	chartList := []store.ReportChart{
		{ChartType: "incidents_severity_bar", Title: "Incidents", IsEnabled: true},
	}
	snapshot := &store.ReportSnapshot{Snapshot: map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339)}}
	items := []store.ReportSnapshotItem{
		{EntityType: "incident", Entity: map[string]any{"severity": "critical"}},
	}
	out, cleanup, err := h.appendChartsToMarkdown(context.Background(), []byte("base"), chartList, snapshot, items, "en", docs.FormatMarkdown)
	if err != nil {
		t.Fatalf("append charts: %v", err)
	}
	cleanup()
	text := string(out)
	if !strings.Contains(text, "data:image/svg+xml") {
		t.Fatalf("expected embedded svg in markdown")
	}
}
