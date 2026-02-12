package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (s *monitoringStore) GetTLS(ctx context.Context, monitorID int64) (*MonitorTLS, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT monitor_id, checked_at, not_after, not_before, common_name, issuer, san_json, fingerprint_sha256, last_error
		FROM monitor_tls WHERE monitor_id=?`, monitorID)
	var tls MonitorTLS
	var sanRaw string
	var lastErr sql.NullString
	if err := row.Scan(&tls.MonitorID, &tls.CheckedAt, &tls.NotAfter, &tls.NotBefore, &tls.CommonName, &tls.Issuer, &sanRaw, &tls.FingerprintSHA256, &lastErr); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if sanRaw != "" {
		_ = json.Unmarshal([]byte(sanRaw), &tls.SANs)
	}
	if lastErr.Valid {
		val := lastErr.String
		tls.LastError = &val
	}
	return &tls, nil
}

func (s *monitoringStore) UpsertTLS(ctx context.Context, tls *MonitorTLS) error {
	if tls == nil {
		return nil
	}
	sanJSON, _ := json.Marshal(tls.SANs)
	res, err := s.db.ExecContext(ctx, `
		UPDATE monitor_tls
		SET checked_at=?, not_after=?, not_before=?, common_name=?, issuer=?, san_json=?, fingerprint_sha256=?, last_error=?
		WHERE monitor_id=?`,
		tls.CheckedAt, tls.NotAfter, tls.NotBefore, tls.CommonName, tls.Issuer, string(sanJSON), tls.FingerprintSHA256, tls.LastError, tls.MonitorID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected > 0 {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO monitor_tls(monitor_id, checked_at, not_after, not_before, common_name, issuer, san_json, fingerprint_sha256, last_error)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		tls.MonitorID, tls.CheckedAt, tls.NotAfter, tls.NotBefore, tls.CommonName, tls.Issuer, string(sanJSON), tls.FingerprintSHA256, tls.LastError)
	return err
}

func (s *monitoringStore) ListCerts(ctx context.Context, filter CertFilter) ([]MonitorCertSummary, error) {
	query := `
		SELECT m.id, m.name, m.url, m.tags_json, COALESCE(s.status, ''), t.checked_at, t.not_after, t.not_before, t.common_name, t.issuer, t.last_error
		FROM monitors m
		LEFT JOIN monitor_state s ON s.monitor_id=m.id
		LEFT JOIN monitor_tls t ON t.monitor_id=m.id
		WHERE LOWER(m.type)='http' AND LOWER(m.url) LIKE 'https:%'`
	var clauses []string
	var args []any
	if len(filter.Tags) > 0 {
		for _, tag := range normalizeMonitorTags(filter.Tags) {
			clauses = append(clauses, "m.tags_json LIKE ?")
			args = append(args, "%"+tag+"%")
		}
	}
	if st := strings.TrimSpace(filter.Status); st != "" {
		clauses = append(clauses, "LOWER(s.status)=?")
		args = append(args, strings.ToLower(st))
	}
	if len(clauses) > 0 {
		query += " AND " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY t.not_after ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		fallbackQuery := `
			SELECT m.id, m.name, m.url, '', COALESCE(s.status, ''), NULL, NULL, NULL, '', '', NULL
			FROM monitors m
			LEFT JOIN monitor_state s ON s.monitor_id=m.id
			WHERE LOWER(m.type)='http' AND LOWER(m.url) LIKE 'https:%'`
		fallbackClauses := clauses
		fallbackArgs := args
		if strings.Contains(strings.ToLower(err.Error()), "tags_json") {
			fallbackClauses = []string{}
			fallbackArgs = []any{}
			if st := strings.TrimSpace(filter.Status); st != "" {
				fallbackClauses = append(fallbackClauses, "LOWER(s.status)=?")
				fallbackArgs = append(fallbackArgs, strings.ToLower(st))
			}
		}
		if len(fallbackClauses) > 0 {
			fallbackQuery += " AND " + strings.Join(fallbackClauses, " AND ")
		}
		fallbackQuery += " ORDER BY m.id ASC"
		rows, err = s.db.QueryContext(ctx, fallbackQuery, fallbackArgs...)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "tags_json") {
				fallbackClauses = []string{}
				fallbackArgs = []any{}
				if st := strings.TrimSpace(filter.Status); st != "" {
					fallbackClauses = append(fallbackClauses, "LOWER(s.status)=?")
					fallbackArgs = append(fallbackArgs, strings.ToLower(st))
				}
				fallbackQuery = `
					SELECT m.id, m.name, m.url, '', COALESCE(s.status, ''), NULL, NULL, NULL, '', '', NULL
					FROM monitors m
					LEFT JOIN monitor_state s ON s.monitor_id=m.id
					WHERE LOWER(m.type)='http' AND LOWER(m.url) LIKE 'https:%'`
				if len(fallbackClauses) > 0 {
					fallbackQuery += " AND " + strings.Join(fallbackClauses, " AND ")
				}
				fallbackQuery += " ORDER BY m.id ASC"
				rows, err = s.db.QueryContext(ctx, fallbackQuery, fallbackArgs...)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	defer rows.Close()
	var res []MonitorCertSummary
	now := time.Now().UTC()
	for rows.Next() {
		var item MonitorCertSummary
		var tagsRaw sql.NullString
		var checkedAt, notAfter, notBefore sql.NullTime
		var lastErr sql.NullString
		if err := rows.Scan(&item.MonitorID, &item.Name, &item.URL, &tagsRaw, &item.Status, &checkedAt, &notAfter, &notBefore, &item.CommonName, &item.Issuer, &lastErr); err != nil {
			return nil, err
		}
		if tagsRaw.Valid && tagsRaw.String != "" {
			_ = json.Unmarshal([]byte(tagsRaw.String), &item.Tags)
		}
		if checkedAt.Valid {
			item.CheckedAt = &checkedAt.Time
		}
		if notAfter.Valid {
			item.NotAfter = &notAfter.Time
			days := int(now.Sub(notAfter.Time).Hours() / -24)
			item.DaysLeft = &days
			if filter.ExpiringLt > 0 && days > filter.ExpiringLt {
				continue
			}
		} else if filter.ExpiringLt > 0 {
			continue
		}
		if notBefore.Valid {
			item.NotBefore = &notBefore.Time
		}
		if lastErr.Valid {
			item.LastError = lastErr.String
		}
		res = append(res, item)
	}
	return res, rows.Err()
}
