package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) CreateColumn(ctx context.Context, column *tasks.Column) (int64, error) {
	if column.Position <= 0 {
		pos, err := s.NextColumnPosition(ctx, column.BoardID)
		if err != nil {
			return 0, err
		}
		column.Position = pos
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_columns(board_id, name, position, is_final, wip_limit, default_template_id, is_active, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		column.BoardID, column.Name, column.Position, boolToInt(column.IsFinal), nullableInt(column.WIPLimit), nullableID(column.DefaultTemplateID), boolToInt(column.IsActive), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	column.ID = id
	column.CreatedAt = now
	column.UpdatedAt = now
	return id, nil
}

func (s *SQLStore) UpdateColumn(ctx context.Context, column *tasks.Column) error {
	now := time.Now().UTC()
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_columns SET name=?, position=?, is_final=?, wip_limit=?, default_template_id=?, is_active=?, updated_at=?
			WHERE id=?`,
			column.Name, column.Position, boolToInt(column.IsFinal), nullableInt(column.WIPLimit), nullableID(column.DefaultTemplateID), boolToInt(column.IsActive), now, column.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET status=?, updated_at=? WHERE column_id=?`, column.Name, now, column.ID); err != nil {
			return err
		}
		return nil
	})
}

func (s *SQLStore) MoveColumn(ctx context.Context, columnID int64, position int) (*tasks.Column, error) {
	var column *tasks.Column
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		column, err = s.getColumnTx(ctx, tx, columnID)
		if err != nil {
			return err
		}
		if column == nil {
			return sql.ErrNoRows
		}
		if position <= 0 {
			row := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_columns WHERE board_id=?`, column.BoardID)
			var max int
			if err := row.Scan(&max); err != nil {
				return err
			}
			position = max
		}
		if position == column.Position {
			return nil
		}
		if position > column.Position {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_columns SET position=position-1
				WHERE board_id=? AND position>? AND position<=?`, column.BoardID, column.Position, position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_columns SET position=position+1
				WHERE board_id=? AND position>=? AND position<?`, column.BoardID, position, column.Position); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE task_columns SET position=?, updated_at=? WHERE id=?`, position, now, columnID); err != nil {
			return err
		}
		column.Position = position
		column.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return column, nil
}

func (s *SQLStore) DeleteColumn(ctx context.Context, columnID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_columns SET is_active=0, updated_at=? WHERE id=?`, time.Now().UTC(), columnID)
	return err
}

func (s *SQLStore) GetColumn(ctx context.Context, columnID int64) (*tasks.Column, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, board_id, name, position, is_final, wip_limit, default_template_id, is_active, created_at, updated_at
		FROM task_columns WHERE id=?`, columnID)
	var c tasks.Column
	var wip sql.NullInt64
	var defaultTemplateID sql.NullInt64
	var isFinal int
	var active int
	if err := row.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &isFinal, &wip, &defaultTemplateID, &active, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if wip.Valid {
		val := int(wip.Int64)
		c.WIPLimit = &val
	}
	if defaultTemplateID.Valid {
		c.DefaultTemplateID = &defaultTemplateID.Int64
	}
	c.IsFinal = isFinal == 1
	c.IsActive = active == 1
	return &c, nil
}

func (s *SQLStore) ListColumns(ctx context.Context, boardID int64, includeInactive bool) ([]tasks.Column, error) {
	query := `
		SELECT id, board_id, name, position, is_final, wip_limit, default_template_id, is_active, created_at, updated_at
		FROM task_columns WHERE board_id=?`
	args := []any{boardID}
	if !includeInactive {
		query += " AND is_active=1"
	}
	query += " ORDER BY position ASC, id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Column
	for rows.Next() {
		var c tasks.Column
		var wip sql.NullInt64
		var defaultTemplateID sql.NullInt64
		var isFinal int
		var active int
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &isFinal, &wip, &defaultTemplateID, &active, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		if wip.Valid {
			val := int(wip.Int64)
			c.WIPLimit = &val
		}
		if defaultTemplateID.Valid {
			c.DefaultTemplateID = &defaultTemplateID.Int64
		}
		c.IsFinal = isFinal == 1
		c.IsActive = active == 1
		res = append(res, c)
	}
	return res, rows.Err()
}

func (s *SQLStore) NextColumnPosition(ctx context.Context, boardID int64) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_columns WHERE board_id=?`, boardID)
	var max int
	if err := row.Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func (s *SQLStore) getColumnTx(ctx context.Context, tx *sql.Tx, columnID int64) (*tasks.Column, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, board_id, name, position, is_final, wip_limit, default_template_id, is_active, created_at, updated_at
		FROM task_columns WHERE id=?`, columnID)
	var c tasks.Column
	var wip sql.NullInt64
	var defaultTemplateID sql.NullInt64
	var isFinal int
	var active int
	if err := row.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &isFinal, &wip, &defaultTemplateID, &active, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if wip.Valid {
		val := int(wip.Int64)
		c.WIPLimit = &val
	}
	if defaultTemplateID.Valid {
		c.DefaultTemplateID = &defaultTemplateID.Int64
	}
	c.IsFinal = isFinal == 1
	c.IsActive = active == 1
	return &c, nil
}
