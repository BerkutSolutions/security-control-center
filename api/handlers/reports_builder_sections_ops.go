package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildControlsSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "controls.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Controls"))
		return res
	}
	if h.controls == nil {
		res.Error = "controls unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 20)
	filter := store.ControlFilter{
		Status:    configString(sec.Config, "status"),
		RiskLevel: configString(sec.Config, "risk"),
		Domain:    configString(sec.Config, "domain"),
		Tag:       configString(sec.Config, "tag"),
	}
	items, err := h.controls.ListControls(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	if len(items) > limit && limit > 0 {
		items = items[:limit]
	}
	statusCounts := map[string]int{}
	riskCounts := map[string]int{}
	failedCount := 0
	for _, c := range items {
		status := strings.ToLower(c.Status)
		statusCounts[status]++
		riskCounts[strings.ToLower(c.RiskLevel)]++
		if status == "failed" || status == "violation" || status == "fail" {
			failedCount++
		}
	}
	res.ItemCount = len(items)
	res.Summary = map[string]any{
		"controls":        len(items),
		"controls_failed": failedCount,
	}
	totals["controls"] += len(items)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Controls")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(items)))
	for key, count := range statusCounts {
		if key == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(key), count))
	}
	if len(items) == 0 {
		b.WriteString("\n_No controls for selected filters._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Code | Title | Status | Risk | Domain |\n|---|---|---|---|---|\n")
	for _, c := range items {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			escapePipes(c.Code),
			escapePipes(c.Title),
			escapePipes(c.Status),
			escapePipes(c.RiskLevel),
			escapePipes(c.Domain),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "control",
			EntityID:   fmt.Sprintf("%d", c.ID),
			Entity: map[string]any{
				"id":         c.ID,
				"code":       c.Code,
				"title":      c.Title,
				"status":     c.Status,
				"risk_level": c.RiskLevel,
				"domain":     c.Domain,
				"created_at": c.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at": c.UpdatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildMonitoringSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "monitoring.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Monitoring"))
		return res
	}
	if h.monitoring == nil {
		res.Error = "monitoring unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 20)
	onlyDown := configBool(sec.Config, "only_down")
	onlyCritical := configBool(sec.Config, "only_critical")
	filter := store.MonitorFilter{}
	if onlyDown {
		filter.Status = "down"
	}
	monitors, err := h.monitoring.ListMonitors(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	var rows []store.MonitorSummary
	downCount := 0
	for _, m := range monitors {
		if onlyCritical && strings.ToLower(m.IncidentSeverity) != "critical" {
			continue
		}
		if strings.ToLower(m.Status) == "down" {
			downCount++
		}
		rows = append(rows, m)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}
	stateByID := map[int64]*store.MonitorState{}
	if len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, m := range rows {
			ids = append(ids, m.ID)
		}
		if states, err := h.monitoring.ListMonitorStates(ctx, ids); err == nil {
			for i := range states {
				state := states[i]
				stateByID[state.MonitorID] = &state
			}
		}
	}
	tlsExpiring := 0
	tlsDays := configInt(sec.Config, "tls_expiring_days", 0)
	if tlsDays > 0 {
		certs, _ := h.monitoring.ListCerts(ctx, store.CertFilter{ExpiringLt: tlsDays})
		tlsExpiring = len(certs)
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"monitors":      len(rows),
		"monitors_down": downCount,
		"tls_expiring":  tlsExpiring,
	}
	totals["monitors"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Monitoring")))
	b.WriteString(fmt.Sprintf("- Total monitors: %d\n", len(rows)))
	b.WriteString(fmt.Sprintf("- Down: %d\n", downCount))
	if tlsDays > 0 {
		b.WriteString(fmt.Sprintf("- TLS expiring (< %d days): %d\n", tlsDays, tlsExpiring))
	}
	if len(rows) == 0 {
		b.WriteString("\n_No monitors for selected filters._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Name | Status | Last Down | Last Error |\n|---|---|---|---|\n")
	for _, m := range rows {
		lastDown := "-"
		if m.LastDownAt != nil {
			lastDown = m.LastDownAt.UTC().Format("2006-01-02 15:04")
		}
		lastErr := m.LastError
		if strings.TrimSpace(lastErr) == "" {
			lastErr = "-"
		}
		state := stateByID[m.ID]
		uptime24 := 0.0
		uptime30 := 0.0
		var tlsLeft any
		if state != nil {
			uptime24 = state.Uptime24h
			uptime30 = state.Uptime30d
			if state.TLSDaysLeft != nil {
				tlsLeft = *state.TLSDaysLeft
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			escapePipes(m.Name),
			escapePipes(m.Status),
			escapePipes(lastDown),
			escapePipes(lastErr),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "monitor",
			EntityID:   fmt.Sprintf("%d", m.ID),
			Entity: map[string]any{
				"id":                m.ID,
				"name":              m.Name,
				"status":            m.Status,
				"incident_severity": m.IncidentSeverity,
				"last_down_at":      lastDown,
				"last_error":        lastErr,
				"uptime_24h":        uptime24,
				"uptime_30d":        uptime30,
				"tls_days_left":     tlsLeft,
			},
		})
	}
	if h.policy.Allowed(roles, "monitoring.events.view") {
		from, _ := periodOverride(sec.Config, fallbackFrom, fallbackTo)
		since := time.Now().AddDate(0, 0, -30).UTC()
		if from != nil {
			since = *from
		}
		evLimit := configInt(sec.Config, "events_limit", 20)
		events, _ := h.monitoring.ListEventsFeed(ctx, store.EventFilter{Since: since, Limit: evLimit})
		if len(events) > 0 {
			b.WriteString("\n### Recent events\n\n| Time | Monitor | Type | Message |\n|---|---|---|---|\n")
			monitorNames := map[int64]string{}
			for _, m := range rows {
				monitorNames[m.ID] = m.Name
			}
			for _, ev := range events {
				name := monitorNames[ev.MonitorID]
				if name == "" {
					name = fmt.Sprintf("#%d", ev.MonitorID)
				}
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
					ev.TS.UTC().Format("2006-01-02 15:04"),
					escapePipes(name),
					escapePipes(ev.EventType),
					escapePipes(ev.Message),
				))
				res.Items = append(res.Items, store.ReportSnapshotItem{
					EntityType: "monitor_event",
					EntityID:   fmt.Sprintf("%d", ev.ID),
					Entity: map[string]any{
						"id":         ev.ID,
						"monitor_id": ev.MonitorID,
						"event_type": ev.EventType,
						"message":    ev.Message,
						"ts":         ev.TS.UTC().Format(time.RFC3339),
					},
				})
			}
		}
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildAuditSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "logs.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Audit events"))
		return res
	}
	if h.audits == nil {
		res.Error = "audit unavailable"
		return res
	}
	limit := configInt(sec.Config, "limit", 50)
	from, _ := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	since := time.Now().AddDate(0, 0, -30).UTC()
	if from != nil {
		since = *from
	}
	records, err := h.audits.ListFiltered(ctx, since, limit*2)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	importantOnly := configBool(sec.Config, "important_only")
	var rows []store.AuditRecord
	for _, rec := range records {
		if importantOnly && !isImportantAudit(rec.Action) {
			continue
		}
		rows = append(rows, rec)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{"audit_events": len(rows)}
	totals["audit_events"] += len(rows)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Audit events")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(rows)))
	if len(rows) == 0 {
		b.WriteString("\n_No audit events for selected period._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| Time | User | Action | Details |\n|---|---|---|---|\n")
	for _, rec := range rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			rec.CreatedAt.UTC().Format("2006-01-02 15:04"),
			escapePipes(rec.Username),
			escapePipes(rec.Action),
			escapePipes(rec.Details),
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "audit",
			EntityID:   fmt.Sprintf("%d", rec.ID),
			Entity: map[string]any{
				"id":         rec.ID,
				"username":   rec.Username,
				"action":     rec.Action,
				"details":    rec.Details,
				"created_at": rec.CreatedAt.UTC().Format(time.RFC3339),
			},
		})
	}
	res.Markdown = b.String()
	return res
}

func (h *ReportsHandler) buildSummarySection(sec store.ReportSection, totals map[string]int, now time.Time) reportSectionResult {
	res := reportSectionResult{Section: sec}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Executive summary")))
	if len(totals) == 0 {
		b.WriteString("_No summary data._\n")
		res.Markdown = b.String()
		return res
	}
	executive := configBool(sec.Config, "executive")
	if executive {
		b.WriteString("### Key KPIs\n\n")
		b.WriteString(fmt.Sprintf("- Critical incidents: %d\n", totals["incidents_critical"]))
		b.WriteString(fmt.Sprintf("- Overdue tasks: %d\n", totals["tasks_overdue"]))
		b.WriteString(fmt.Sprintf("- Control violations: %d\n", totals["controls_failed"]))
		b.WriteString(fmt.Sprintf("- Monitoring downtime: %d\n", totals["monitors_down"]))
		b.WriteString(fmt.Sprintf("- TLS expiring: %d\n\n", totals["tls_expiring"]))
		if totals["incidents_critical"]+totals["incidents_high"] > 0 {
			b.WriteString("### Top risks\n\n")
			b.WriteString(fmt.Sprintf("- Critical/high incidents present: %d\n\n", totals["incidents_critical"]+totals["incidents_high"]))
		}
	}
	if v := totals["incidents"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Incidents in period: %d\n", v))
	}
	if v := totals["tasks"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Tasks touched: %d\n", v))
	}
	if v := totals["tasks_overdue"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Overdue tasks: %d\n", v))
	}
	if v := totals["docs"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Documents updated: %d\n", v))
	}
	if v := totals["controls"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Controls in scope: %d\n", v))
	}
	if v := totals["monitors"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Monitors tracked: %d\n", v))
	}
	if v := totals["audit_events"]; v > 0 {
		b.WriteString(fmt.Sprintf("- Audit events: %d\n", v))
	}
	res.Markdown = b.String()
	return res
}

func isImportantAudit(action string) bool {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return false
	}
	importantPrefixes := []string{
		"incident.", "incidents.", "report.", "reports.", "docs.", "tasks.",
		"controls.", "monitoring.", "accounts.", "approval.", "auth.",
	}
	for _, prefix := range importantPrefixes {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	if strings.Contains(action, "delete") || strings.Contains(action, "export") || strings.Contains(action, "create") {
		return true
	}
	return false
}
