package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildIncidentsSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, eff store.EffectiveAccess, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "incidents.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Incidents"))
		return res
	}
	from, to := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	limit := configInt(sec.Config, "limit", 20)
	filter := store.IncidentFilter{
		Status:   configString(sec.Config, "status"),
		Severity: configString(sec.Config, "severity"),
		Limit:    limit * 5,
	}
	items, err := h.incidents.ListIncidents(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	incidentType := strings.TrimSpace(configString(sec.Config, "type"))
	ownerFilter := configInt(sec.Config, "owner", 0)
	tagsFilter := configStrings(sec.Config, "tags")
	ownerCache := map[int64]string{}
	var rows []store.Incident
	for _, inc := range items {
		if from != nil || to != nil {
			if !withinPeriod(inc.CreatedAt, from, to) {
				continue
			}
		}
		if incidentType != "" && !strings.EqualFold(inc.Meta.IncidentType, incidentType) {
			continue
		}
		if ownerFilter > 0 && int64(ownerFilter) != inc.OwnerUserID {
			continue
		}
		if len(tagsFilter) > 0 {
			found := false
			for _, t := range inc.Meta.Tags {
				for _, want := range tagsFilter {
					if strings.EqualFold(t, want) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}
		if !h.canViewIncidentByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
			continue
		}
		acl, _ := h.incidents.GetIncidentACL(ctx, inc.ID)
		if !h.incidentsSvc.CheckACL(user, roles, acl, "view") {
			continue
		}
		rows = append(rows, inc)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}
	statusCounts := map[string]int{}
	severityCounts := map[string]int{}
	for _, inc := range rows {
		statusCounts[strings.ToLower(inc.Status)]++
		severityCounts[strings.ToLower(inc.Severity)]++
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"incidents":          len(rows),
		"incidents_critical": severityCounts["critical"],
		"incidents_high":     severityCounts["high"],
	}
	totals["incidents"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Incidents")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(rows)))
	for key, count := range statusCounts {
		if key == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(key), count))
	}
	if len(rows) == 0 {
		b.WriteString("\n_No incidents for selected period._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| ID | Title | Severity | Status | Owner | Created |\n|---|---|---|---|---|---|\n")
	for _, inc := range rows {
		ownerName := h.cachedUserName(ownerCache, inc.OwnerUserID)
		idLabel := inc.RegNo
		if strings.TrimSpace(idLabel) == "" {
			idLabel = fmt.Sprintf("%d", inc.ID)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			escapePipes(idLabel),
			escapePipes(inc.Title),
			escapePipes(inc.Severity),
			escapePipes(inc.Status),
			escapePipes(ownerName),
			inc.CreatedAt.UTC().Format("2006-01-02"),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "incident",
			EntityID:   fmt.Sprintf("%d", inc.ID),
			Entity: map[string]any{
				"id":                   inc.ID,
				"reg_no":               inc.RegNo,
				"title":                inc.Title,
				"severity":             inc.Severity,
				"status":               inc.Status,
				"owner_user_id":        inc.OwnerUserID,
				"assignee_user_id":     inc.AssigneeUserID,
				"created_at":           inc.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at":           inc.UpdatedAt.UTC().Format(time.RFC3339),
				"classification_level": inc.ClassificationLevel,
				"classification_tags":  inc.ClassificationTags,
			},
		})
	}
	res.Markdown = b.String()
	return res
}
