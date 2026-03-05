package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *monitoringStore) GetMonitorState(ctx context.Context, id int64) (*MonitorState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT monitor_id, status, last_result_status, maintenance_active, retry_at, retry_attempt, last_attempt_at, last_error_kind, last_checked_at, last_up_at, last_down_at, last_latency_ms, last_status_code, last_error, uptime_24h, uptime_30d, avg_latency_24h, tls_days_left, tls_not_after, incident_score, incident_score_updated_at, incident_score_reasons, incident_score_posterior, incident_score_state, incident_score_observation
		FROM monitor_state WHERE monitor_id=?`, id)
	return scanMonitorState(row)
}

func (s *monitoringStore) ListMonitorStates(ctx context.Context, ids []int64) ([]MonitorState, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := `
		SELECT monitor_id, status, last_result_status, maintenance_active, retry_at, retry_attempt, last_attempt_at, last_error_kind, last_checked_at, last_up_at, last_down_at, last_latency_ms, last_status_code, last_error, uptime_24h, uptime_30d, avg_latency_24h, tls_days_left, tls_not_after, incident_score, incident_score_updated_at, incident_score_reasons, incident_score_posterior, incident_score_state, incident_score_observation
		FROM monitor_state WHERE monitor_id IN (` + placeholders(len(ids)) + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MonitorState
	for rows.Next() {
		state, err := scanMonitorState(rows)
		if err != nil {
			return nil, err
		}
		if state != nil {
			res = append(res, *state)
		}
	}
	return res, rows.Err()
}

func (s *monitoringStore) UpsertMonitorState(ctx context.Context, st *MonitorState) error {
	reasonsJSON := "[]"
	if st != nil && len(st.IncidentScoreReasons) > 0 {
		if b, err := json.Marshal(st.IncidentScoreReasons); err == nil {
			reasonsJSON = string(b)
		}
	}
	posteriorJSON := "[]"
	if st != nil && len(st.IncidentScorePosterior) > 0 {
		if b, err := json.Marshal(st.IncidentScorePosterior); err == nil {
			posteriorJSON = string(b)
		}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_state(monitor_id, status, last_result_status, maintenance_active, retry_at, retry_attempt, last_attempt_at, last_error_kind, last_checked_at, last_up_at, last_down_at, last_latency_ms, last_status_code, last_error, uptime_24h, uptime_30d, avg_latency_24h, tls_days_left, tls_not_after, incident_score, incident_score_updated_at, incident_score_reasons, incident_score_posterior, incident_score_state, incident_score_observation)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (monitor_id)
		DO UPDATE SET
			status=excluded.status,
			last_result_status=excluded.last_result_status,
			maintenance_active=excluded.maintenance_active,
			retry_at=excluded.retry_at,
			retry_attempt=excluded.retry_attempt,
			last_attempt_at=excluded.last_attempt_at,
			last_error_kind=excluded.last_error_kind,
			last_checked_at=excluded.last_checked_at,
			last_up_at=excluded.last_up_at,
			last_down_at=excluded.last_down_at,
			last_latency_ms=excluded.last_latency_ms,
			last_status_code=excluded.last_status_code,
			last_error=excluded.last_error,
			uptime_24h=excluded.uptime_24h,
			uptime_30d=excluded.uptime_30d,
			avg_latency_24h=excluded.avg_latency_24h,
			tls_days_left=excluded.tls_days_left,
			tls_not_after=excluded.tls_not_after,
			incident_score=excluded.incident_score,
			incident_score_updated_at=excluded.incident_score_updated_at,
			incident_score_reasons=excluded.incident_score_reasons,
			incident_score_posterior=excluded.incident_score_posterior,
			incident_score_state=excluded.incident_score_state,
			incident_score_observation=excluded.incident_score_observation`,
		st.MonitorID, st.Status, st.LastResultStatus, boolToInt(st.MaintenanceActive), st.RetryAt, st.RetryAttempt, st.LastAttemptAt, st.LastErrorKind,
		st.LastCheckedAt, st.LastUpAt, st.LastDownAt, st.LastLatencyMs, st.LastStatusCode, st.LastError,
		st.Uptime24h, st.Uptime30d, st.AvgLatency24h, st.TLSDaysLeft, st.TLSNotAfter,
		st.IncidentScore, st.IncidentScoreUpdatedAt, reasonsJSON, posteriorJSON, st.IncidentScoreState, st.IncidentScoreObs)
	return err
}

func (s *monitoringStore) MarkMonitorDueNow(ctx context.Context, monitorID int64) error {
	// Ensures the monitor becomes due for checks ASAP (engine loop will pick it up).
	// We do not touch any other state fields, only reset last_checked_at.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_state(monitor_id, status, retry_at, retry_attempt, last_checked_at)
		VALUES(?, ?, NULL, 0, NULL)
		ON CONFLICT (monitor_id)
		DO UPDATE SET last_checked_at=NULL, retry_at=NULL, retry_attempt=0`,
		monitorID, "down",
	)
	return err
}

func (s *monitoringStore) AddMetric(ctx context.Context, metric *MonitorMetric) (int64, error) {
	hdrJSON := ""
	if metric != nil && len(metric.RespHdrs) > 0 {
		if b, err := json.Marshal(metric.RespHdrs); err == nil {
			hdrJSON = string(b)
		}
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_metrics(monitor_id, ts, latency_ms, ok, status_code, error, final_url, remote_ip, response_headers_json)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		metric.MonitorID, metric.TS, metric.LatencyMs, boolToInt(metric.OK), metric.StatusCode, metric.Error,
		strings.TrimSpace(metric.FinalURL), strings.TrimSpace(metric.RemoteIP), hdrJSON)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *monitoringStore) ListMetrics(ctx context.Context, monitorID int64, since time.Time) ([]MonitorMetric, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, monitor_id, ts, latency_ms, ok, status_code, error, final_url, remote_ip, response_headers_json
		FROM monitor_metrics WHERE monitor_id=? AND ts>=? ORDER BY ts ASC`, monitorID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []MonitorMetric
	for rows.Next() {
		var m MonitorMetric
		var okInt int
		var status sql.NullInt64
		var errText sql.NullString
		var finalURL sql.NullString
		var remoteIP sql.NullString
		var hdrJSON sql.NullString
		if err := rows.Scan(&m.ID, &m.MonitorID, &m.TS, &m.LatencyMs, &okInt, &status, &errText, &finalURL, &remoteIP, &hdrJSON); err != nil {
			return nil, err
		}
		m.OK = okInt == 1
		if status.Valid {
			val := int(status.Int64)
			m.StatusCode = &val
		}
		if errText.Valid {
			val := errText.String
			m.Error = &val
		}
		if finalURL.Valid {
			m.FinalURL = strings.TrimSpace(finalURL.String)
		}
		if remoteIP.Valid {
			m.RemoteIP = strings.TrimSpace(remoteIP.String)
		}
		if hdrJSON.Valid && strings.TrimSpace(hdrJSON.String) != "" {
			var headers map[string]string
			if err := json.Unmarshal([]byte(hdrJSON.String), &headers); err == nil && len(headers) > 0 {
				m.RespHdrs = headers
			}
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

func (s *monitoringStore) ListEvents(ctx context.Context, monitorID int64, since time.Time) ([]MonitorEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, monitor_id, ts, event_type, message
		FROM monitor_events WHERE monitor_id=? AND ts>=? ORDER BY ts DESC`, monitorID, since)
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

func (s *monitoringStore) AddEvent(ctx context.Context, event *MonitorEvent) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO monitor_events(monitor_id, ts, event_type, message)
		VALUES(?,?,?,?)`, event.MonitorID, event.TS, event.EventType, event.Message)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *monitoringStore) MetricsSummary(ctx context.Context, monitorID int64, since time.Time) (int, int, float64, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN ok=1 THEN 1 ELSE 0 END), AVG(CASE WHEN ok=1 THEN latency_ms END)
		FROM monitor_metrics WHERE monitor_id=? AND ts>=?`, monitorID, since)
	var total, okCount sql.NullInt64
	var avg sql.NullFloat64
	if err := row.Scan(&total, &okCount, &avg); err != nil {
		return 0, 0, 0, err
	}
	if !total.Valid {
		return 0, 0, 0, nil
	}
	totalVal := int(total.Int64)
	okVal := 0
	if okCount.Valid {
		okVal = int(okCount.Int64)
	}
	avgVal := 0.0
	if avg.Valid {
		avgVal = avg.Float64
	}
	return okVal, totalVal, avgVal, nil
}

func (s *monitoringStore) MetricsSummaryBetween(ctx context.Context, monitorID int64, since, until time.Time) (int, int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), SUM(CASE WHEN ok=1 THEN 1 ELSE 0 END)
		FROM monitor_metrics
		WHERE monitor_id=? AND ts>=? AND ts<?`, monitorID, since, until)
	var total, okCount sql.NullInt64
	if err := row.Scan(&total, &okCount); err != nil {
		return 0, 0, err
	}
	totalVal := 0
	okVal := 0
	if total.Valid {
		totalVal = int(total.Int64)
	}
	if okCount.Valid {
		okVal = int(okCount.Int64)
	}
	return okVal, totalVal, nil
}

func (s *monitoringStore) DeleteMetricsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_metrics WHERE ts < ?`, before)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func (s *monitoringStore) DeleteMonitorMetrics(ctx context.Context, monitorID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_metrics WHERE monitor_id=?`, monitorID)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func (s *monitoringStore) DeleteMonitorEvents(ctx context.Context, monitorID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitor_events WHERE monitor_id=?`, monitorID)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func scanMonitorState(row interface {
	Scan(dest ...any) error
}) (*MonitorState, error) {
	var st MonitorState
	var retryAt, lastAttemptAt sql.NullTime
	var retryAttempt sql.NullInt64
	var lastErrorKind sql.NullString
	var lastChecked, lastUp, lastDown sql.NullTime
	var lastLatency, lastStatus sql.NullInt64
	var maintenanceInt sql.NullInt64
	var tlsDays sql.NullInt64
	var tlsNotAfter sql.NullTime
	var incidentScore sql.NullFloat64
	var incidentScoreUpdatedAt sql.NullTime
	var incidentScoreReasons sql.NullString
	var incidentScorePosterior sql.NullString
	var incidentScoreState sql.NullString
	var incidentScoreObs sql.NullString
	if err := row.Scan(
		&st.MonitorID, &st.Status, &st.LastResultStatus, &maintenanceInt, &retryAt, &retryAttempt, &lastAttemptAt, &lastErrorKind,
		&lastChecked, &lastUp, &lastDown, &lastLatency, &lastStatus, &st.LastError,
		&st.Uptime24h, &st.Uptime30d, &st.AvgLatency24h, &tlsDays, &tlsNotAfter,
		&incidentScore, &incidentScoreUpdatedAt, &incidentScoreReasons, &incidentScorePosterior, &incidentScoreState, &incidentScoreObs,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if maintenanceInt.Valid {
		st.MaintenanceActive = maintenanceInt.Int64 == 1
	}
	if lastChecked.Valid {
		st.LastCheckedAt = &lastChecked.Time
	}
	if retryAt.Valid {
		t := retryAt.Time.UTC()
		st.RetryAt = &t
	}
	if retryAttempt.Valid {
		st.RetryAttempt = int(retryAttempt.Int64)
	}
	if lastAttemptAt.Valid {
		t := lastAttemptAt.Time.UTC()
		st.LastAttemptAt = &t
	}
	if lastErrorKind.Valid {
		st.LastErrorKind = lastErrorKind.String
	}
	if lastUp.Valid {
		st.LastUpAt = &lastUp.Time
	}
	if lastDown.Valid {
		st.LastDownAt = &lastDown.Time
	}
	if lastLatency.Valid {
		val := int(lastLatency.Int64)
		st.LastLatencyMs = &val
	}
	if lastStatus.Valid {
		val := int(lastStatus.Int64)
		st.LastStatusCode = &val
	}
	if tlsDays.Valid {
		val := int(tlsDays.Int64)
		st.TLSDaysLeft = &val
	}
	if tlsNotAfter.Valid {
		st.TLSNotAfter = &tlsNotAfter.Time
	}
	if incidentScore.Valid {
		val := incidentScore.Float64
		st.IncidentScore = &val
	}
	if incidentScoreUpdatedAt.Valid {
		t := incidentScoreUpdatedAt.Time.UTC()
		st.IncidentScoreUpdatedAt = &t
	}
	if incidentScoreReasons.Valid {
		var reasons []string
		if err := json.Unmarshal([]byte(incidentScoreReasons.String), &reasons); err == nil {
			st.IncidentScoreReasons = reasons
		}
	}
	if incidentScorePosterior.Valid && incidentScorePosterior.String != "" {
		var post []float64
		if err := json.Unmarshal([]byte(incidentScorePosterior.String), &post); err == nil {
			st.IncidentScorePosterior = post
		}
	}
	if incidentScoreState.Valid {
		st.IncidentScoreState = incidentScoreState.String
	}
	if incidentScoreObs.Valid {
		st.IncidentScoreObs = incidentScoreObs.String
	}
	return &st, nil
}
