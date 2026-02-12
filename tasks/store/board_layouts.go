package store

import (
	"context"
	"database/sql"
	"time"
)

func (s *SQLiteStore) GetBoardLayout(ctx context.Context, userID, spaceID int64) (string, error) {
	if userID == 0 || spaceID == 0 {
		return "", nil
	}
	var layout string
	err := s.db.QueryRowContext(ctx, `
		SELECT layout_json
		FROM task_board_layouts
		WHERE user_id=? AND space_id=?
	`, userID, spaceID).Scan(&layout)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return layout, err
}

func (s *SQLiteStore) SaveBoardLayout(ctx context.Context, userID, spaceID int64, layoutJSON string) error {
	if userID == 0 || spaceID == 0 {
		return nil
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_board_layouts (user_id, space_id, layout_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, space_id)
		DO UPDATE SET layout_json=excluded.layout_json, updated_at=excluded.updated_at
	`, userID, spaceID, layoutJSON, now, now)
	return err
}
