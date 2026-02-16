package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/store"
)

type reportSectionResult struct {
	Section   store.ReportSection
	Markdown  string
	Summary   map[string]any
	Items     []store.ReportSnapshotItem
	Denied    bool
	Error     string
	ItemCount int
}

func (h *ReportsHandler) buildReportSections(ctx context.Context, doc *store.Document, meta *store.ReportMeta, sections []store.ReportSection, user *store.User, roles []string, groups []store.Group, eff store.EffectiveAccess) ([]reportSectionResult, []store.ReportSnapshotItem, map[string]any, error) {
	now := time.Now().UTC()
	var periodFrom, periodTo *time.Time
	if meta != nil {
		periodFrom = meta.PeriodFrom
		periodTo = meta.PeriodTo
	}
	totals := map[string]int{}
	results := make([]reportSectionResult, 0, len(sections))
	summaryIndex := -1
	for i, sec := range sections {
		sec.SectionType = strings.ToLower(strings.TrimSpace(sec.SectionType))
		res := reportSectionResult{Section: sec}
		if !sec.IsEnabled {
			results = append(results, res)
			continue
		}
		switch sec.SectionType {
		case "summary":
			summaryIndex = i
			results = append(results, res)
			continue
		case "incidents":
			res = h.buildIncidentsSection(ctx, sec, user, roles, eff, periodFrom, periodTo, totals)
		case "tasks":
			res = h.buildTasksSection(ctx, sec, user, roles, groups, periodFrom, periodTo, totals)
		case "docs":
			res = h.buildDocsSection(ctx, sec, user, roles, periodFrom, periodTo, totals)
		case "controls":
			res = h.buildControlsSection(ctx, sec, user, roles, totals)
		case "monitoring":
			res = h.buildMonitoringSection(ctx, sec, user, roles, periodFrom, periodTo, totals)
		case "sla_summary":
			res = h.buildSLASummarySection(ctx, sec, user, roles, periodFrom, periodTo, totals)
		case "audit":
			res = h.buildAuditSection(ctx, sec, user, roles, periodFrom, periodTo, totals)
		case "custom_md":
			res = h.buildCustomSection(sec)
		default:
			res.Error = "unsupported section"
		}
		if res.Summary != nil {
			for k, v := range res.Summary {
				if num, ok := v.(int); ok {
					totals[k] += num
				}
			}
		}
		results = append(results, res)
	}
	if summaryIndex >= 0 && summaryIndex < len(results) {
		results[summaryIndex] = h.buildSummarySection(sections[summaryIndex], totals, now)
	}
	var items []store.ReportSnapshotItem
	var sectionsPayload []map[string]any
	for _, res := range results {
		sec := res.Section
		sectionsPayload = append(sectionsPayload, map[string]any{
			"type":       sec.SectionType,
			"title":      sec.Title,
			"enabled":    sec.IsEnabled,
			"denied":     res.Denied,
			"error":      res.Error,
			"summary":    res.Summary,
			"item_count": res.ItemCount,
		})
		if len(res.Items) > 0 {
			items = append(items, res.Items...)
		}
	}
	snapshot := map[string]any{
		"report_id":    doc.ID,
		"generated_at": now.Format(time.RFC3339),
		"period_from":  formatPeriodTime(periodFrom),
		"period_to":    formatPeriodTime(periodTo),
		"sections":     sectionsPayload,
	}
	return results, items, snapshot, nil
}

func applySectionContent(mode string, baseContent string, sections []reportSectionResult) string {
	if strings.ToLower(strings.TrimSpace(mode)) != "markers" {
		var blocks []string
		for _, res := range sections {
			if !res.Section.IsEnabled || strings.TrimSpace(res.Markdown) == "" {
				continue
			}
			blocks = append(blocks, wrapSectionMarkdown(markerKeyForSection(res.Section), res.Markdown))
		}
		return strings.TrimSpace(strings.Join(blocks, "\n\n"))
	}
	content := baseContent
	for _, res := range sections {
		if !res.Section.IsEnabled || strings.TrimSpace(res.Markdown) == "" {
			continue
		}
		content = replaceSectionBlock(content, markerKeyForSection(res.Section), res.Markdown)
	}
	return strings.TrimSpace(content)
}

func wrapSectionMarkdown(sectionType string, md string) string {
	return fmt.Sprintf("<!-- SECTION: %s -->\n%s\n<!-- ENDSECTION -->", sectionType, strings.TrimSpace(md))
}

func replaceSectionBlock(content string, sectionType string, md string) string {
	startMarker := fmt.Sprintf("<!-- SECTION: %s -->", sectionType)
	endMarker := "<!-- ENDSECTION -->"
	start := strings.Index(content, startMarker)
	if start >= 0 {
		rest := content[start+len(startMarker):]
		end := strings.Index(rest, endMarker)
		if end >= 0 {
			endPos := start + len(startMarker) + end + len(endMarker)
			return strings.TrimRight(content[:start], "\n") + "\n\n" + wrapSectionMarkdown(sectionType, md) + "\n\n" + strings.TrimLeft(content[endPos:], "\n")
		}
	}
	if strings.TrimSpace(content) == "" {
		return wrapSectionMarkdown(sectionType, md)
	}
	return strings.TrimRight(content, "\n") + "\n\n" + wrapSectionMarkdown(sectionType, md)
}

func markerKeyForSection(sec store.ReportSection) string {
	if strings.ToLower(strings.TrimSpace(sec.SectionType)) != "custom_md" {
		return sec.SectionType
	}
	key := configString(sec.Config, "key")
	if key == "" {
		return sec.SectionType
	}
	return fmt.Sprintf("%s:%s", sec.SectionType, key)
}

func formatPeriodTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func configString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	val, ok := cfg[key]
	if !ok {
		return ""
	}
	switch v := val.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func configBool(cfg map[string]any, key string) bool {
	if cfg == nil {
		return false
	}
	val, ok := cfg[key]
	if !ok {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		return strings.ToLower(strings.TrimSpace(v)) == "true"
	default:
		return false
	}
}

func configInt(cfg map[string]any, key string, def int) int {
	if cfg == nil {
		return def
	}
	val, ok := cfg[key]
	if !ok {
		return def
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func configStrings(cfg map[string]any, key string) []string {
	if cfg == nil {
		return nil
	}
	val, ok := cfg[key]
	if !ok {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []any:
		var out []string
		for _, item := range v {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		return splitCSV(v)
	default:
		return nil
	}
}

func periodOverride(cfg map[string]any, fallbackFrom, fallbackTo *time.Time) (*time.Time, *time.Time) {
	from := fallbackFrom
	to := fallbackTo
	if cfg == nil {
		return from, to
	}
	if raw := configString(cfg, "period_from"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			t := parsed.UTC()
			from = &t
		}
	}
	if raw := configString(cfg, "period_to"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			t := parsed.UTC()
			to = &t
		}
	}
	return from, to
}

func withinPeriod(ts time.Time, from, to *time.Time) bool {
	if from != nil && ts.Before(*from) {
		return false
	}
	if to != nil && ts.After(*to) {
		return false
	}
	return true
}
