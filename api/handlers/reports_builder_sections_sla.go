package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *ReportsHandler) buildSLASummarySection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !h.policy.Allowed(roles, "monitoring.view") {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "SLA executive summary"))
		return res
	}
	if h.monitoring == nil {
		res.Error = "monitoring unavailable"
		return res
	}

	periodType := strings.ToLower(strings.TrimSpace(configString(sec.Config, "period_type")))
	if periodType != "day" && periodType != "week" && periodType != "month" {
		periodType = "month"
	}
	limit := configInt(sec.Config, "limit", 50)
	onlyViolations := configBool(sec.Config, "only_violations")
	includeCurrent := true
	if _, ok := sec.Config["include_current"]; ok {
		includeCurrent = configBool(sec.Config, "include_current")
	}
	from, to := periodOverride(sec.Config, fallbackFrom, fallbackTo)

	filter := store.MonitorSLAPeriodResultListFilter{
		PeriodType:   periodType,
		Limit:        max(limit*4, 100),
		OnlyViolates: onlyViolations,
	}
	periodRows, err := h.monitoring.ListSLAPeriodResults(ctx, filter)
	if err != nil {
		periodRows = nil
	}

	monitors, err := h.monitoring.ListMonitors(ctx, store.MonitorFilter{})
	if err != nil {
		res.Error = "load failed"
		return res
	}
	monitorByID := make(map[int64]store.MonitorSummary, len(monitors))
	for _, m := range monitors {
		monitorByID[m.ID] = m
	}

	var rows []store.MonitorSLAPeriodResult
	for _, row := range periodRows {
		if from != nil || to != nil {
			if !withinPeriod(row.PeriodEnd, from, to) {
				continue
			}
		}
		rows = append(rows, row)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}

	violations := 0
	okCount := 0
	unknown := 0
	sumUptime := 0.0
	avgCount := 0
	violatedMonitors := map[int64]struct{}{}
	for _, row := range rows {
		switch row.Status {
		case "violated":
			violations++
			violatedMonitors[row.MonitorID] = struct{}{}
		case "ok":
			okCount++
		default:
			unknown++
		}
		if row.Status == "ok" || row.Status == "violated" {
			sumUptime += row.UptimePct
			avgCount++
		}
	}
	avgUptime := 0.0
	if avgCount > 0 {
		avgUptime = sumUptime / float64(avgCount)
	}
	violationRate := 0.0
	if len(rows) > 0 {
		violationRate = float64(violations) * 100 / float64(len(rows))
	}

	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"sla_periods":           len(rows),
		"sla_violations":        violations,
		"sla_unknown":           unknown,
		"sla_monitors_violated": len(violatedMonitors),
	}
	totals["sla_periods"] += len(rows)
	totals["sla_violations"] += violations

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "SLA executive summary")))
	b.WriteString(fmt.Sprintf("- Period type: %s\n", strings.Title(periodType)))
	b.WriteString(fmt.Sprintf("- Evaluated periods: %d\n", len(rows)))
	b.WriteString(fmt.Sprintf("- Violations: %d (%.2f%%)\n", violations, violationRate))
	b.WriteString(fmt.Sprintf("- SLA OK: %d\n", okCount))
	b.WriteString(fmt.Sprintf("- Insufficient data: %d\n", unknown))
	b.WriteString(fmt.Sprintf("- Avg uptime (known): %.2f%%\n", avgUptime))
	b.WriteString(fmt.Sprintf("- Monitors with violations: %d\n", len(violatedMonitors)))

	if len(rows) == 0 {
		b.WriteString("\n_No closed SLA periods for selected filter._\n")
	} else {
		b.WriteString("\n### Closed SLA periods\n\n")
		b.WriteString("| Monitor | Window | Uptime | Coverage | Target | Status | Incident |\n|---|---|---:|---:|---:|---|---|\n")
		for _, row := range rows {
			name := fmt.Sprintf("#%d", row.MonitorID)
			if mon, ok := monitorByID[row.MonitorID]; ok && strings.TrimSpace(mon.Name) != "" {
				name = mon.Name
			}
			b.WriteString(fmt.Sprintf("| %s | %s - %s | %.2f%% | %.2f%% | %.2f%% | %s | %s |\n",
				escapePipes(name),
				row.PeriodStart.UTC().Format("2006-01-02"),
				row.PeriodEnd.UTC().Format("2006-01-02"),
				row.UptimePct,
				row.CoveragePct,
				row.TargetPct,
				escapePipes(strings.ToUpper(row.Status)),
				boolWord(row.IncidentCreated),
			))
			res.Items = append(res.Items, store.ReportSnapshotItem{
				EntityType: "monitor_sla_period",
				EntityID:   fmt.Sprintf("%d", row.ID),
				Entity: map[string]any{
					"id":               row.ID,
					"monitor_id":       row.MonitorID,
					"period_type":      row.PeriodType,
					"period_start":     row.PeriodStart.UTC().Format(time.RFC3339),
					"period_end":       row.PeriodEnd.UTC().Format(time.RFC3339),
					"uptime_pct":       row.UptimePct,
					"coverage_pct":     row.CoveragePct,
					"target_pct":       row.TargetPct,
					"status":           row.Status,
					"incident_created": row.IncidentCreated,
				},
			})
		}
	}

	if includeCurrent {
		settings, _ := h.monitoring.GetSettings(ctx)
		targetDefault := 90.0
		if settings != nil && settings.DefaultSLATargetPct > 0 && settings.DefaultSLATargetPct <= 100 {
			targetDefault = settings.DefaultSLATargetPct
		}
		ids := make([]int64, 0, len(monitors))
		for _, m := range monitors {
			ids = append(ids, m.ID)
		}
		states, _ := h.monitoring.ListMonitorStates(ctx, ids)
		sort.Slice(states, func(i, j int) bool {
			return states[i].Uptime30d < states[j].Uptime30d
		})
		top := states
		if len(top) > 10 {
			top = top[:10]
		}
		if len(top) > 0 {
			b.WriteString("\n### Current trend (24h/30d)\n\n")
			b.WriteString("| Monitor | Uptime 24h | Uptime 30d | Target |\n|---|---:|---:|---:|\n")
			for _, st := range top {
				name := fmt.Sprintf("#%d", st.MonitorID)
				target := targetDefault
				if mon, ok := monitorByID[st.MonitorID]; ok {
					if strings.TrimSpace(mon.Name) != "" {
						name = mon.Name
					}
					if mon.SLATargetPct != nil && *mon.SLATargetPct > 0 && *mon.SLATargetPct <= 100 {
						target = *mon.SLATargetPct
					}
				}
				b.WriteString(fmt.Sprintf("| %s | %.2f%% | %.2f%% | %.2f%% |\n",
					escapePipes(name),
					st.Uptime24h,
					st.Uptime30d,
					target,
				))
			}
		}
	}

	res.Markdown = b.String()
	return res
}

func boolWord(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
