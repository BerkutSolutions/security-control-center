package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	defaultSLAIncidentPeriod = "day"
	defaultSLAMinCoveragePct = 80.0
)

func (s *monitoringStore) GetMonitorSLAPolicy(ctx context.Context, monitorID int64) (*MonitorSLAPolicy, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT monitor_id, incident_on_violation, incident_period, min_coverage_pct, updated_at
		FROM monitor_sla_policies
		WHERE monitor_id=?`, monitorID)
	item, err := scanMonitorSLAPolicy(row)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return defaultSLAPolicyFor(monitorID), nil
	}
	return item, nil
}

func (s *monitoringStore) UpsertMonitorSLAPolicy(ctx context.Context, policy *MonitorSLAPolicy) error {
	normalized := normalizeSLAPolicy(policy)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_sla_policies(monitor_id, incident_on_violation, incident_period, min_coverage_pct, updated_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT (monitor_id)
		DO UPDATE SET
			incident_on_violation=excluded.incident_on_violation,
			incident_period=excluded.incident_period,
			min_coverage_pct=excluded.min_coverage_pct,
			updated_at=excluded.updated_at
	`, normalized.MonitorID, boolToInt(normalized.IncidentOnViolation), normalized.IncidentPeriod, normalized.MinCoveragePct, normalized.UpdatedAt)
	return err
}

func (s *monitoringStore) ListMonitorSLAPolicies(ctx context.Context, monitorIDs []int64) ([]MonitorSLAPolicy, error) {
	if len(monitorIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(monitorIDs))
	for _, id := range monitorIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT monitor_id, incident_on_violation, incident_period, min_coverage_pct, updated_at
		FROM monitor_sla_policies
		WHERE monitor_id IN (`+placeholders(len(monitorIDs))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorSLAPolicy
	for rows.Next() {
		item, err := scanMonitorSLAPolicy(rows)
		if err != nil {
			return nil, err
		}
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, rows.Err()
}

func (s *monitoringStore) UpsertSLAPeriodResult(ctx context.Context, item *MonitorSLAPeriodResult) (*MonitorSLAPeriodResult, error) {
	normalized := normalizeSLAPeriodResult(item)
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO monitor_sla_period_results(
			monitor_id, period_type, period_start, period_end,
			uptime_pct, coverage_pct, target_pct, status, incident_created, created_at, updated_at
		)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (monitor_id, period_type, period_start)
		DO UPDATE SET
			period_end=excluded.period_end,
			uptime_pct=excluded.uptime_pct,
			coverage_pct=excluded.coverage_pct,
			target_pct=excluded.target_pct,
			status=excluded.status,
			updated_at=excluded.updated_at
		RETURNING id, monitor_id, period_type, period_start, period_end,
			uptime_pct, coverage_pct, target_pct, status, incident_created, created_at, updated_at
	`, normalized.MonitorID, normalized.PeriodType, normalized.PeriodStart, normalized.PeriodEnd,
		normalized.UptimePct, normalized.CoveragePct, normalized.TargetPct, normalized.Status,
		boolToInt(normalized.IncidentCreated), normalized.CreatedAt, normalized.UpdatedAt)
	return scanMonitorSLAPeriodResult(row)
}

func (s *monitoringStore) ListSLAPeriodResults(ctx context.Context, filter MonitorSLAPeriodResultListFilter) ([]MonitorSLAPeriodResult, error) {
	query := `
		SELECT id, monitor_id, period_type, period_start, period_end,
			uptime_pct, coverage_pct, target_pct, status, incident_created, created_at, updated_at
		FROM monitor_sla_period_results`
	var clauses []string
	var args []any
	if filter.MonitorID != nil && *filter.MonitorID > 0 {
		clauses = append(clauses, "monitor_id=?")
		args = append(args, *filter.MonitorID)
	}
	if periodType := normalizeSLAIncidentPeriod(filter.PeriodType); periodType != "" {
		clauses = append(clauses, "period_type=?")
		args = append(args, periodType)
	}
	if status := normalizeSLAStatus(filter.Status); status != "" {
		clauses = append(clauses, "status=?")
		args = append(args, status)
	}
	if filter.OnlyViolates {
		clauses = append(clauses, "status='violated'")
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY period_end DESC, id DESC"
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query += " LIMIT " + intToString(limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonitorSLAPeriodResult
	for rows.Next() {
		item, err := scanMonitorSLAPeriodResult(rows)
		if err != nil {
			return nil, err
		}
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, rows.Err()
}

func (s *monitoringStore) MarkSLAPeriodIncidentCreated(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitor_sla_period_results
		SET incident_created=1, updated_at=?
		WHERE id=?`, time.Now().UTC(), id)
	return err
}

func (s *monitoringStore) SyncSLAPeriodTarget(ctx context.Context, monitorID int64, targetPct float64, minCoveragePct float64) error {
	if monitorID <= 0 {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitor_sla_period_results
		SET
			target_pct=?,
			status=CASE
				WHEN coverage_pct < ? THEN 'unknown'
				WHEN uptime_pct >= ? THEN 'ok'
				ELSE 'violated'
			END,
			updated_at=?
		WHERE monitor_id=?
	`, targetPct, minCoveragePct, targetPct, now, monitorID)
	return err
}

func defaultSLAPolicyFor(monitorID int64) *MonitorSLAPolicy {
	return &MonitorSLAPolicy{
		MonitorID:           monitorID,
		IncidentOnViolation: false,
		IncidentPeriod:      defaultSLAIncidentPeriod,
		MinCoveragePct:      defaultSLAMinCoveragePct,
		UpdatedAt:           time.Now().UTC(),
	}
}

func normalizeSLAPolicy(policy *MonitorSLAPolicy) *MonitorSLAPolicy {
	out := defaultSLAPolicyFor(0)
	if policy != nil {
		*out = *policy
	}
	out.IncidentPeriod = normalizeSLAIncidentPeriod(out.IncidentPeriod)
	if out.IncidentPeriod == "" {
		out.IncidentPeriod = defaultSLAIncidentPeriod
	}
	if out.MinCoveragePct <= 0 || out.MinCoveragePct > 100 {
		out.MinCoveragePct = defaultSLAMinCoveragePct
	}
	out.UpdatedAt = time.Now().UTC()
	return out
}

func normalizeSLAPeriodResult(item *MonitorSLAPeriodResult) *MonitorSLAPeriodResult {
	now := time.Now().UTC()
	out := &MonitorSLAPeriodResult{
		CreatedAt: now,
		UpdatedAt: now,
	}
	if item != nil {
		*out = *item
	}
	out.PeriodType = normalizeSLAIncidentPeriod(out.PeriodType)
	out.Status = normalizeSLAStatus(out.Status)
	if out.Status == "" {
		out.Status = "unknown"
	}
	if out.CreatedAt.IsZero() {
		out.CreatedAt = now
	}
	out.UpdatedAt = now
	return out
}

func normalizeSLAIncidentPeriod(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "day", "week", "month":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeSLAStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ok", "violated", "unknown":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func scanMonitorSLAPolicy(row interface{ Scan(dest ...any) error }) (*MonitorSLAPolicy, error) {
	var item MonitorSLAPolicy
	var incidentInt int
	if err := row.Scan(&item.MonitorID, &incidentInt, &item.IncidentPeriod, &item.MinCoveragePct, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.IncidentOnViolation = incidentInt == 1
	item.IncidentPeriod = normalizeSLAIncidentPeriod(item.IncidentPeriod)
	if item.IncidentPeriod == "" {
		item.IncidentPeriod = defaultSLAIncidentPeriod
	}
	if item.MinCoveragePct <= 0 || item.MinCoveragePct > 100 {
		item.MinCoveragePct = defaultSLAMinCoveragePct
	}
	return &item, nil
}

func scanMonitorSLAPeriodResult(row interface{ Scan(dest ...any) error }) (*MonitorSLAPeriodResult, error) {
	var item MonitorSLAPeriodResult
	var incidentInt int
	if err := row.Scan(
		&item.ID, &item.MonitorID, &item.PeriodType, &item.PeriodStart, &item.PeriodEnd,
		&item.UptimePct, &item.CoveragePct, &item.TargetPct, &item.Status, &incidentInt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.IncidentCreated = incidentInt == 1
	item.PeriodType = normalizeSLAIncidentPeriod(item.PeriodType)
	if item.PeriodType == "" {
		item.PeriodType = "day"
	}
	item.Status = normalizeSLAStatus(item.Status)
	if item.Status == "" {
		item.Status = "unknown"
	}
	return &item, nil
}
