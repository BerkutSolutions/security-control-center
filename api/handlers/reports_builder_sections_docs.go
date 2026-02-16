package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildDocsSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "docs.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Documents"))
		return res
	}
	from, to := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	limit := configInt(sec.Config, "limit", 20)
	filter := store.DocumentFilter{
		Status:  configString(sec.Config, "status"),
		Tags:    configStrings(sec.Config, "tags"),
		Limit:   limit * 5,
		DocType: "document",
	}
	docsList, err := h.docs.ListDocuments(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	classFilter := strings.TrimSpace(configString(sec.Config, "classification"))
	var classLevel *int
	if classFilter != "" {
		if level, err := docs.ParseLevel(classFilter); err == nil {
			val := int(level)
			classLevel = &val
		}
	}
	statusCounts := map[string]int{}
	var rows []store.Document
	for _, d := range docsList {
		if classLevel != nil && d.ClassificationLevel != *classLevel {
			continue
		}
		if from != nil || to != nil {
			if !withinPeriod(d.CreatedAt, from, to) {
				continue
			}
		}
		docACL, _ := h.docs.GetDocACL(ctx, d.ID)
		var folderACL []store.ACLRule
		if d.FolderID != nil {
			folderACL, _ = h.docs.GetFolderACL(ctx, *d.FolderID)
		}
		if !h.svc.CheckACL(user, roles, &d, docACL, folderACL, "view") {
			continue
		}
		rows = append(rows, d)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}
	approvalStatus := map[int64]string{}
	if len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, d := range rows {
			ids = append(ids, d.ID)
		}
		if approvals, err := h.docs.ListApprovalsByDocIDs(ctx, ids); err == nil {
			for _, ap := range approvals {
				if _, ok := approvalStatus[ap.DocID]; ok {
					continue
				}
				approvalStatus[ap.DocID] = strings.ToLower(strings.TrimSpace(ap.Status))
			}
		}
	}
	for _, d := range rows {
		statusCounts[strings.ToLower(d.Status)]++
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{"docs": len(rows)}
	totals["docs"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Documents")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(rows)))
	for key, count := range statusCounts {
		if key == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(key), count))
	}
	if len(rows) == 0 {
		b.WriteString("\n_No documents for selected period._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Reg # | Title | Status | Classification | Updated |\n|---|---|---|---|---|\n")
	for _, d := range rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			escapePipes(d.RegNumber),
			escapePipes(d.Title),
			escapePipes(d.Status),
			escapePipes(docs.LevelName(docs.ClassificationLevel(d.ClassificationLevel))),
			d.UpdatedAt.UTC().Format("2006-01-02"),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "doc",
			EntityID:   fmt.Sprintf("%d", d.ID),
			Entity: map[string]any{
				"id":                   d.ID,
				"reg_number":           d.RegNumber,
				"title":                d.Title,
				"status":               d.Status,
				"classification_level": d.ClassificationLevel,
				"classification_tags":  d.ClassificationTags,
				"created_at":           d.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at":           d.UpdatedAt.UTC().Format(time.RFC3339),
				"approval_status":      approvalStatus[d.ID],
			},
		})
	}
	res.Markdown = b.String()
	return res
}
