package monitoring

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"berkut-scc/core/store"
)

type SLAEvaluation struct {
	UptimePct   float64
	CoveragePct float64
	TargetPct   float64
	Status      string
}

func (e *Engine) runSLAEvaluator(ctx context.Context, settings store.MonitorSettings) {
	e.mu.Lock()
	last := e.lastSLAAt
	e.mu.Unlock()
	if !last.IsZero() && time.Since(last) < time.Minute {
		return
	}
	now := time.Now().UTC()
	periods := []struct {
		kind  string
		start time.Time
		end   time.Time
	}{
		{kind: "day", start: startOfUTCDay(now).Add(-24 * time.Hour), end: startOfUTCDay(now)},
		{kind: "week", start: startOfUTCWeek(now).Add(-7 * 24 * time.Hour), end: startOfUTCWeek(now)},
		{kind: "month", start: startOfUTCMonth(now).AddDate(0, -1, 0), end: startOfUTCMonth(now)},
	}
	for _, item := range periods {
		e.evaluateSLAPeriod(ctx, settings, item.kind, item.start, item.end)
	}
	e.mu.Lock()
	e.lastSLAAt = now
	e.mu.Unlock()
}

func (e *Engine) evaluateSLAPeriod(ctx context.Context, settings store.MonitorSettings, periodType string, periodStart, periodEnd time.Time) {
	if e.store == nil {
		return
	}
	monitors, err := e.store.ListMonitors(ctx, store.MonitorFilter{})
	if err != nil {
		if e.logger != nil {
			e.logger.Errorf("monitoring sla list monitors: %v", err)
		}
		return
	}
	if len(monitors) == 0 {
		return
	}
	ids := make([]int64, 0, len(monitors))
	for _, mon := range monitors {
		ids = append(ids, mon.ID)
	}
	policies, err := e.store.ListMonitorSLAPolicies(ctx, ids)
	if err != nil {
		if e.logger != nil {
			e.logger.Errorf("monitoring sla list policies: %v", err)
		}
		return
	}
	policyMap := make(map[int64]store.MonitorSLAPolicy, len(policies))
	for _, item := range policies {
		policyMap[item.MonitorID] = item
	}
	for _, mon := range monitors {
		policy := defaultPolicyForMonitor(mon.ID)
		if existing, ok := policyMap[mon.ID]; ok {
			policy = existing
		}
		eval, err := e.EvaluateMonitorSLAWindow(ctx, mon.Monitor, policy, settings, periodStart, periodEnd)
		if err != nil {
			if e.logger != nil {
				e.logger.Errorf("monitoring sla evaluate monitor %d: %v", mon.ID, err)
			}
			continue
		}
		result, err := e.store.UpsertSLAPeriodResult(ctx, &store.MonitorSLAPeriodResult{
			MonitorID:       mon.ID,
			PeriodType:      periodType,
			PeriodStart:     periodStart,
			PeriodEnd:       periodEnd,
			UptimePct:       eval.UptimePct,
			CoveragePct:     eval.CoveragePct,
			TargetPct:       eval.TargetPct,
			Status:          eval.Status,
			IncidentCreated: false,
		})
		if err != nil {
			if e.logger != nil {
				e.logger.Errorf("monitoring sla upsert result monitor %d: %v", mon.ID, err)
			}
			continue
		}
		e.createSLAIncidentIfNeeded(ctx, mon.Monitor, policy, result)
	}
}

func (e *Engine) EvaluateMonitorSLAWindow(ctx context.Context, monitor store.Monitor, policy store.MonitorSLAPolicy, settings store.MonitorSettings, periodStart, periodEnd time.Time) (SLAEvaluation, error) {
	metrics, err := e.store.ListMetrics(ctx, monitor.ID, periodStart)
	if err != nil {
		return SLAEvaluation{}, err
	}
	windows, err := e.store.MaintenanceWindowsFor(ctx, monitor.ID, monitor.Tags, periodStart, periodEnd)
	if err != nil {
		return SLAEvaluation{}, err
	}
	maintenanceSeconds := 0.0
	for _, item := range windows {
		if item.End.After(item.Start) {
			maintenanceSeconds += item.End.Sub(item.Start).Seconds()
		}
	}
	windowSeconds := periodEnd.Sub(periodStart).Seconds() - maintenanceSeconds
	if windowSeconds < 0 {
		windowSeconds = 0
	}
	interval := monitor.IntervalSec
	if interval <= 0 {
		interval = settings.DefaultIntervalSec
	}
	if interval <= 0 {
		interval = 60
	}
	expectedChecks := int(math.Ceil(windowSeconds / float64(interval)))
	if expectedChecks < 1 {
		expectedChecks = 1
	}
	okCount := 0
	totalCount := 0
	for _, metric := range metrics {
		if metric.TS.Before(periodStart) || !metric.TS.Before(periodEnd) {
			continue
		}
		if tsInsideWindows(metric.TS, windows) {
			continue
		}
		totalCount++
		if metric.OK {
			okCount++
		}
	}
	coverage := (float64(totalCount) / float64(expectedChecks)) * 100.0
	if coverage > 100 {
		coverage = 100
	}
	uptime := 0.0
	if totalCount > 0 {
		uptime = (float64(okCount) / float64(totalCount)) * 100.0
	}
	target := effectiveSLATarget(monitor, settings)
	status := "unknown"
	if coverage >= policy.MinCoveragePct && totalCount > 0 {
		status = "violated"
		if uptime >= target {
			status = "ok"
		}
	}
	return SLAEvaluation{
		UptimePct:   roundTwo(uptime),
		CoveragePct: roundTwo(coverage),
		TargetPct:   roundTwo(target),
		Status:      status,
	}, nil
}

func tsInsideWindows(ts time.Time, windows []store.MaintenanceWindow) bool {
	for _, item := range windows {
		if (ts.After(item.Start) || ts.Equal(item.Start)) && ts.Before(item.End) {
			return true
		}
	}
	return false
}

func (e *Engine) createSLAIncidentIfNeeded(ctx context.Context, monitor store.Monitor, policy store.MonitorSLAPolicy, result *store.MonitorSLAPeriodResult) {
	if e.incidents == nil || result == nil {
		return
	}
	if result.IncidentCreated || !policy.IncidentOnViolation || policy.IncidentPeriod != result.PeriodType || result.Status != "violated" {
		return
	}
	owner := monitor.CreatedBy
	if owner <= 0 {
		owner = 1
	}
	title := fmt.Sprintf("SLA violation: %s", monitorDisplayName(monitor))
	desc := fmt.Sprintf(
		"SLA period closed with violation. Monitor: %s. Period: %s - %s. Uptime: %.2f%%. Coverage: %.2f%%. Target: %.2f%%.",
		monitorDisplayName(monitor),
		result.PeriodStart.Format(time.RFC3339),
		result.PeriodEnd.Format(time.RFC3339),
		result.UptimePct,
		result.CoveragePct,
		result.TargetPct,
	)
	sourceRef := result.ID
	incident := &store.Incident{
		Title:       title,
		Description: desc,
		Severity:    "medium",
		Status:      "open",
		OwnerUserID: owner,
		CreatedBy:   owner,
		UpdatedBy:   owner,
		Source:      "monitoring_sla",
		SourceRefID: &sourceRef,
		Meta: store.IncidentMeta{
			IncidentType:          "SLA breach",
			DetectionSource:       "Monitoring SLA evaluator",
			SLAResponse:           "1h",
			FirstResponseDeadline: "8h",
			WhatHappened:          "SLA threshold violated on period close",
			DetectedAt:            result.PeriodEnd.Format(time.RFC3339),
			AffectedSystems:       monitorDisplayName(monitor),
			Risk:                  "yes",
			ActionsTaken:          "Incident created automatically by SLA evaluator",
		},
	}
	id, err := e.incidents.CreateIncident(ctx, incident, nil, nil, e.incidentRegFormat)
	if err != nil {
		if e.logger != nil {
			e.logger.Errorf("monitoring sla incident create monitor=%d result=%d: %v", monitor.ID, result.ID, err)
		}
		return
	}
	_ = e.store.MarkSLAPeriodIncidentCreated(ctx, result.ID)
	_, _ = e.incidents.AddIncidentTimeline(ctx, &store.IncidentTimelineEvent{
		IncidentID: id,
		EventType:  "monitoring.sla.auto_create",
		Message:    monitorDisplayName(monitor),
		CreatedBy:  owner,
		EventAt:    time.Now().UTC(),
	})
	if e.audits != nil {
		_ = e.audits.Log(ctx, "system", "monitoring.sla.incident.auto_create", fmt.Sprintf("incident_id=%d result_id=%d monitor_id=%d", id, result.ID, monitor.ID))
	}
}

func effectiveSLATarget(monitor store.Monitor, settings store.MonitorSettings) float64 {
	if monitor.SLATargetPct != nil && *monitor.SLATargetPct > 0 && *monitor.SLATargetPct <= 100 {
		return *monitor.SLATargetPct
	}
	if settings.DefaultSLATargetPct > 0 && settings.DefaultSLATargetPct <= 100 {
		return settings.DefaultSLATargetPct
	}
	return 90
}

func defaultPolicyForMonitor(monitorID int64) store.MonitorSLAPolicy {
	return store.MonitorSLAPolicy{
		MonitorID:           monitorID,
		IncidentOnViolation: false,
		IncidentPeriod:      "day",
		MinCoveragePct:      80,
		UpdatedAt:           time.Now().UTC(),
	}
}

func startOfUTCDay(ts time.Time) time.Time {
	y, m, d := ts.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func startOfUTCWeek(ts time.Time) time.Time {
	day := startOfUTCDay(ts)
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, -(weekday - 1))
}

func startOfUTCMonth(ts time.Time) time.Time {
	y, m, _ := ts.UTC().Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

func monitorDisplayName(monitor store.Monitor) string {
	name := strings.TrimSpace(monitor.Name)
	if name != "" {
		return name
	}
	return fmt.Sprintf("Monitor #%d", monitor.ID)
}

func roundTwo(v float64) float64 {
	return math.Round(v*100) / 100
}
