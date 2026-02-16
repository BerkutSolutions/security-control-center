package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

type ReportChart struct {
	ID         int64          `json:"id"`
	ReportID   int64          `json:"report_id"`
	ChartType  string         `json:"chart_type"`
	Title      string         `json:"title"`
	SectionType string        `json:"section_type"`
	Config     map[string]any `json:"config,omitempty"`
	OrderIndex int            `json:"order_index"`
	IsEnabled  bool           `json:"is_enabled"`
}

func (s *reportsStore) ListReportCharts(ctx context.Context, reportID int64) ([]ReportChart, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, report_id, chart_type, title, section_type, config_json, order_index, is_enabled
		FROM report_charts WHERE report_id=? ORDER BY order_index ASC, id ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ReportChart
	for rows.Next() {
		var ch ReportChart
		var cfg string
		var enabled int
		if err := rows.Scan(&ch.ID, &ch.ReportID, &ch.ChartType, &ch.Title, &ch.SectionType, &cfg, &ch.OrderIndex, &enabled); err != nil {
			return nil, err
		}
		ch.IsEnabled = enabled == 1
		if cfg != "" {
			_ = json.Unmarshal([]byte(cfg), &ch.Config)
		}
		res = append(res, ch)
	}
	return res, rows.Err()
}

func (s *reportsStore) GetReportChart(ctx context.Context, chartID int64) (*ReportChart, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, report_id, chart_type, title, section_type, config_json, order_index, is_enabled
		FROM report_charts WHERE id=?`, chartID)
	var ch ReportChart
	var cfg string
	var enabled int
	if err := row.Scan(&ch.ID, &ch.ReportID, &ch.ChartType, &ch.Title, &ch.SectionType, &cfg, &ch.OrderIndex, &enabled); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ch.IsEnabled = enabled == 1
	if cfg != "" {
		_ = json.Unmarshal([]byte(cfg), &ch.Config)
	}
	return &ch, nil
}

func (s *reportsStore) ReplaceReportCharts(ctx context.Context, reportID int64, charts []ReportChart) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM report_charts WHERE report_id=?`, reportID); err != nil {
		tx.Rollback()
		return err
	}
	for i, ch := range charts {
		cfg, _ := json.Marshal(ch.Config)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO report_charts(report_id, chart_type, title, section_type, config_json, order_index, is_enabled)
			VALUES(?,?,?,?,?,?,?)`,
			reportID, ch.ChartType, ch.Title, ch.SectionType, string(cfg), i, boolToInt(ch.IsEnabled)); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
