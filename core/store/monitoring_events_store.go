package store

import (
	"context"
	"strconv"
	"strings"
)

func (s *monitoringStore) ListEventsFeed(ctx context.Context, filter EventFilter) ([]MonitorEvent, error) {
	query := `
		SELECT e.id, e.monitor_id, e.ts, e.event_type, e.message
		FROM monitor_events e
		INNER JOIN monitors m ON m.id=e.monitor_id`
	var clauses []string
	var args []any
	if !filter.Since.IsZero() {
		clauses = append(clauses, "e.ts>=?")
		args = append(args, filter.Since)
	}
	if filter.MonitorID != nil {
		clauses = append(clauses, "e.monitor_id=?")
		args = append(args, *filter.MonitorID)
	}
	if len(filter.Types) > 0 {
		var typeClauses []string
		for _, t := range filter.Types {
			val := strings.ToLower(strings.TrimSpace(t))
			if val == "" {
				continue
			}
			typeClauses = append(typeClauses, "LOWER(e.event_type)=?")
			args = append(args, val)
		}
		if len(typeClauses) > 0 {
			clauses = append(clauses, "("+strings.Join(typeClauses, " OR ")+")")
		}
	}
	if len(filter.Tags) > 0 {
		for _, tag := range normalizeMonitorTags(filter.Tags) {
			clauses = append(clauses, "m.tags_json LIKE ?")
			args = append(args, "%"+tag+"%")
		}
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY e.ts DESC"
	if filter.Limit > 0 {
		query += " LIMIT " + strconv.Itoa(filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MonitorEvent
	for rows.Next() {
		var e MonitorEvent
		if err := rows.Scan(&e.ID, &e.MonitorID, &e.TS, &e.EventType, &e.Message); err != nil {
			return nil, err
		}
		res = append(res, e)
	}
	return res, rows.Err()
}
