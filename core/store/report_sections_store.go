package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type ReportSection struct {
	ID          int64           `json:"id"`
	ReportID    int64           `json:"report_id"`
	SectionType string          `json:"section_type"`
	Title       string          `json:"title"`
	Config      map[string]any  `json:"config,omitempty"`
	OrderIndex  int             `json:"order_index"`
	IsEnabled   bool            `json:"is_enabled"`
}

type ReportSnapshot struct {
	ID        int64          `json:"id"`
	ReportID  int64          `json:"report_id"`
	CreatedAt time.Time      `json:"created_at"`
	CreatedBy int64          `json:"created_by"`
	Reason    string         `json:"reason"`
	Snapshot  map[string]any `json:"snapshot,omitempty"`
	SnapshotJSON string      `json:"-"`
	Sha256    string         `json:"sha256"`
}

type ReportSnapshotItem struct {
	ID         int64          `json:"id"`
	SnapshotID int64          `json:"snapshot_id"`
	EntityType string         `json:"entity_type"`
	EntityID   string         `json:"entity_id"`
	Entity     map[string]any `json:"entity,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (s *reportsStore) ListReportSections(ctx context.Context, reportID int64) ([]ReportSection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, report_id, section_type, title, config_json, order_index, is_enabled
		FROM report_sections WHERE report_id=? ORDER BY order_index ASC, id ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportSection
	for rows.Next() {
		var sec ReportSection
		var cfg string
		var enabled int
		if err := rows.Scan(&sec.ID, &sec.ReportID, &sec.SectionType, &sec.Title, &cfg, &sec.OrderIndex, &enabled); err != nil {
			return nil, err
		}
		sec.IsEnabled = enabled == 1
		if cfg != "" {
			_ = json.Unmarshal([]byte(cfg), &sec.Config)
		}
		res = append(res, sec)
	}
	return res, rows.Err()
}

func (s *reportsStore) ReplaceReportSections(ctx context.Context, reportID int64, sections []ReportSection) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_sections WHERE report_id=?`, reportID); err != nil {
		tx.Rollback()
		return err
	}
	for i, sec := range sections {
		cfg, _ := json.Marshal(sec.Config)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO report_sections(report_id, section_type, title, config_json, order_index, is_enabled)
			VALUES(?,?,?,?,?,?)`,
			reportID, sec.SectionType, sec.Title, string(cfg), i, boolToInt(sec.IsEnabled)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *reportsStore) CreateReportSnapshot(ctx context.Context, snapshot *ReportSnapshot, items []ReportSnapshotItem) (int64, error) {
	if snapshot == nil {
		return 0, errors.New("missing snapshot")
	}
	payload := []byte(snapshot.SnapshotJSON)
	if len(payload) == 0 {
		payload, _ = json.Marshal(snapshot.Snapshot)
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO report_snapshots(report_id, created_at, created_by, reason, snapshot_json, sha256)
		VALUES(?,?,?,?,?,?)`,
		snapshot.ReportID, snapshot.CreatedAt, nullableID(&snapshot.CreatedBy), snapshot.Reason, string(payload), snapshot.Sha256)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	snapshotID, _ := res.LastInsertId()
	for _, item := range items {
		entityJSON, _ := json.Marshal(item.Entity)
		createdAt := item.CreatedAt
		if createdAt.IsZero() {
			createdAt = snapshot.CreatedAt
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO report_snapshot_items(snapshot_id, entity_type, entity_id, entity_json, created_at)
			VALUES(?,?,?,?,?)`,
			snapshotID, item.EntityType, item.EntityID, string(entityJSON), createdAt); err != nil {
			tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	snapshot.ID = snapshotID
	return snapshotID, nil
}

func (s *reportsStore) ListReportSnapshots(ctx context.Context, reportID int64) ([]ReportSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, report_id, created_at, created_by, reason, snapshot_json, sha256
		FROM report_snapshots WHERE report_id=? ORDER BY created_at DESC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportSnapshot
	for rows.Next() {
		var snap ReportSnapshot
		var payload string
		var createdBy sql.NullInt64
		if err := rows.Scan(&snap.ID, &snap.ReportID, &snap.CreatedAt, &createdBy, &snap.Reason, &payload, &snap.Sha256); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			snap.CreatedBy = createdBy.Int64
		}
		snap.SnapshotJSON = payload
		if payload != "" {
			_ = json.Unmarshal([]byte(payload), &snap.Snapshot)
		}
		res = append(res, snap)
	}
	return res, rows.Err()
}

func (s *reportsStore) GetReportSnapshot(ctx context.Context, snapshotID int64) (*ReportSnapshot, []ReportSnapshotItem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, report_id, created_at, created_by, reason, snapshot_json, sha256
		FROM report_snapshots WHERE id=?`, snapshotID)
	var snap ReportSnapshot
	var payload string
	var createdBy sql.NullInt64
	if err := row.Scan(&snap.ID, &snap.ReportID, &snap.CreatedAt, &createdBy, &snap.Reason, &payload, &snap.Sha256); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if createdBy.Valid {
		snap.CreatedBy = createdBy.Int64
	}
	snap.SnapshotJSON = payload
	if payload != "" {
		_ = json.Unmarshal([]byte(payload), &snap.Snapshot)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, snapshot_id, entity_type, entity_id, entity_json, created_at
		FROM report_snapshot_items WHERE snapshot_id=? ORDER BY id ASC`, snapshotID)
	if err != nil {
		return &snap, nil, err
	}
	defer rows.Close()
	var items []ReportSnapshotItem
	for rows.Next() {
		var item ReportSnapshotItem
		var entityJSON string
		if err := rows.Scan(&item.ID, &item.SnapshotID, &item.EntityType, &item.EntityID, &entityJSON, &item.CreatedAt); err != nil {
			return &snap, nil, err
		}
		if entityJSON != "" {
			_ = json.Unmarshal([]byte(entityJSON), &item.Entity)
		}
		items = append(items, item)
	}
	return &snap, items, rows.Err()
}
