package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

func (s *monitoringStore) ListMaintenance(ctx context.Context, filter MaintenanceFilter) ([]MonitorMaintenance, error) {
	query := `
		SELECT id, name, monitor_id, tags_json, starts_at, ends_at, timezone, is_recurring, rrule_text, created_by, created_at, updated_at, is_active
		FROM monitor_maintenance`
	var clauses []string
	var args []any
	if filter.Active != nil {
		clauses = append(clauses, "is_active=?")
		args = append(args, boolToInt(*filter.Active))
	}
	if filter.MonitorID != nil {
		clauses = append(clauses, "(monitor_id IS NULL OR monitor_id=?)")
		args = append(args, *filter.MonitorID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY starts_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MonitorMaintenance
	for rows.Next() {
		item, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *item)
	}
	return res, rows.Err()
}

func (s *monitoringStore) GetMaintenance(ctx context.Context, id int64) (*MonitorMaintenance, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, monitor_id, tags_json, starts_at, ends_at, timezone, is_recurring, rrule_text, created_by, created_at, updated_at, is_active
		FROM monitor_maintenance WHERE id=?`, id)
	item, err := scanMaintenance(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func (s *monitoringStore) CreateMaintenance(ctx context.Context, m *MonitorMaintenance) (int64, error) {
	now := time.Now().UTC()
	tagsJSON := tagsToJSON(normalizeMonitorTags(m.Tags))
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_maintenance(name, monitor_id, tags_json, starts_at, ends_at, timezone, is_recurring, rrule_text, created_by, created_at, updated_at, is_active)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		strings.TrimSpace(m.Name), nullableID(m.MonitorID), tagsJSON, m.StartsAt, m.EndsAt, strings.TrimSpace(m.Timezone),
		boolToInt(m.IsRecurring), strings.TrimSpace(m.RRuleText), m.CreatedBy, now, now, boolToInt(m.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *monitoringStore) UpdateMaintenance(ctx context.Context, m *MonitorMaintenance) error {
	tagsJSON := tagsToJSON(normalizeMonitorTags(m.Tags))
	_, err := s.db.ExecContext(ctx, `
		UPDATE monitor_maintenance
		SET name=?, monitor_id=?, tags_json=?, starts_at=?, ends_at=?, timezone=?, is_recurring=?, rrule_text=?, updated_at=?, is_active=?
		WHERE id=?`,
		strings.TrimSpace(m.Name), nullableID(m.MonitorID), tagsJSON, m.StartsAt, m.EndsAt, strings.TrimSpace(m.Timezone),
		boolToInt(m.IsRecurring), strings.TrimSpace(m.RRuleText), time.Now().UTC(), boolToInt(m.IsActive), m.ID)
	return err
}

func (s *monitoringStore) DeleteMaintenance(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM monitor_maintenance WHERE id=?`, id)
	return err
}

func (s *monitoringStore) ActiveMaintenanceFor(ctx context.Context, monitorID int64, tags []string, now time.Time) ([]MonitorMaintenance, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, monitor_id, tags_json, starts_at, ends_at, timezone, is_recurring, rrule_text, created_by, created_at, updated_at, is_active
		FROM monitor_maintenance WHERE is_active=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MonitorMaintenance
	tagSet := map[string]struct{}{}
	for _, t := range normalizeMonitorTags(tags) {
		tagSet[t] = struct{}{}
	}
	for rows.Next() {
		item, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		if item.MonitorID != nil && *item.MonitorID != monitorID {
			continue
		}
		if len(item.Tags) > 0 && !hasAnyTag(tagSet, item.Tags) {
			continue
		}
		if maintenanceActiveAt(*item, now) {
			res = append(res, *item)
		}
	}
	return res, rows.Err()
}

func scanMaintenance(row interface {
	Scan(dest ...any) error
}) (*MonitorMaintenance, error) {
	var m MonitorMaintenance
	var tagsRaw sql.NullString
	var monitorID sql.NullInt64
	var recurring, active int
	if err := row.Scan(&m.ID, &m.Name, &monitorID, &tagsRaw, &m.StartsAt, &m.EndsAt, &m.Timezone, &recurring, &m.RRuleText, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt, &active); err != nil {
		return nil, err
	}
	if tagsRaw.Valid && tagsRaw.String != "" {
		_ = json.Unmarshal([]byte(tagsRaw.String), &m.Tags)
	}
	if monitorID.Valid {
		m.MonitorID = &monitorID.Int64
	}
	m.IsRecurring = recurring == 1
	m.IsActive = active == 1
	return &m, nil
}

func hasAnyTag(set map[string]struct{}, tags []string) bool {
	for _, t := range normalizeMonitorTags(tags) {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

type rruleSpec struct {
	freq     string
	interval int
	byDay    []time.Weekday
}

func parseRRule(raw string) (*rruleSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty rrule")
	}
	parts := strings.Split(raw, ";")
	spec := &rruleSpec{interval: 1}
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return nil, errors.New("invalid rrule")
		}
		key := strings.ToUpper(strings.TrimSpace(kv[0]))
		val := strings.ToUpper(strings.TrimSpace(kv[1]))
		switch key {
		case "FREQ":
			spec.freq = val
		case "INTERVAL":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return nil, errors.New("invalid interval")
			}
			spec.interval = n
		case "BYDAY":
			if val == "" {
				continue
			}
			days := strings.Split(val, ",")
			for _, d := range days {
				if wd, ok := parseWeekday(d); ok {
					spec.byDay = append(spec.byDay, wd)
				} else {
					return nil, errors.New("invalid weekday")
				}
			}
		default:
			return nil, errors.New("invalid rrule")
		}
	}
	if spec.freq != "DAILY" && spec.freq != "WEEKLY" {
		return nil, errors.New("invalid rrule")
	}
	return spec, nil
}

func parseWeekday(raw string) (time.Weekday, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MO":
		return time.Monday, true
	case "TU":
		return time.Tuesday, true
	case "WE":
		return time.Wednesday, true
	case "TH":
		return time.Thursday, true
	case "FR":
		return time.Friday, true
	case "SA":
		return time.Saturday, true
	case "SU":
		return time.Sunday, true
	default:
		return time.Sunday, false
	}
}

func maintenanceActiveAt(m MonitorMaintenance, now time.Time) bool {
	if !m.IsRecurring {
		return (now.After(m.StartsAt) || now.Equal(m.StartsAt)) && now.Before(m.EndsAt)
	}
	spec, err := parseRRule(m.RRuleText)
	if err != nil {
		return false
	}
	if spec.freq == "WEEKLY" && len(spec.byDay) == 0 {
		spec.byDay = []time.Weekday{m.StartsAt.Weekday()}
	}
	startDay := dateOnly(m.StartsAt)
	today := dateOnly(now)
	switch spec.freq {
	case "DAILY":
		days := int(today.Sub(startDay).Hours() / 24)
		if days < 0 || days%spec.interval != 0 {
			return false
		}
	case "WEEKLY":
		weeks := int(today.Sub(startDay).Hours()/24) / 7
		if weeks < 0 || weeks%spec.interval != 0 {
			return false
		}
		if len(spec.byDay) > 0 && !weekdayAllowed(now.Weekday(), spec.byDay) {
			return false
		}
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), m.StartsAt.Hour(), m.StartsAt.Minute(), m.StartsAt.Second(), 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day(), m.EndsAt.Hour(), m.EndsAt.Minute(), m.EndsAt.Second(), 0, time.UTC)
	if end.Before(start) {
		end = end.Add(24 * time.Hour)
	}
	return (now.After(start) || now.Equal(start)) && now.Before(end)
}

func weekdayAllowed(day time.Weekday, list []time.Weekday) bool {
	for _, d := range list {
		if d == day {
			return true
		}
	}
	return false
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
