package store

import (
	"context"
	"database/sql"
	"time"
)

type DashboardStore interface {
	GetLayout(ctx context.Context, userID int64) (string, error)
	SaveLayout(ctx context.Context, userID int64, layoutJSON string) error
}

type dashboardStore struct {
	db *sql.DB
}

func NewDashboardStore(db *sql.DB) DashboardStore {
	return &dashboardStore{db: db}
}

func (s *dashboardStore) GetLayout(ctx context.Context, userID int64) (string, error) {
	var layout string
	err := s.db.QueryRowContext(ctx, `SELECT layout_json FROM dashboard_layouts WHERE user_id=?`, userID).Scan(&layout)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return layout, err
}

func (s *dashboardStore) SaveLayout(ctx context.Context, userID int64, layoutJSON string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO dashboard_layouts(user_id, layout_json, created_at, updated_at)
		VALUES(?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET layout_json=excluded.layout_json, updated_at=excluded.updated_at`,
		userID, layoutJSON, now, now)
	return err
}
