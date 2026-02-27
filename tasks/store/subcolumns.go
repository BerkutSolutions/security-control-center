package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) CreateSubColumn(ctx context.Context, subcolumn *tasks.SubColumn) (int64, error) {
	if subcolumn.Position <= 0 {
		pos, err := s.NextSubColumnPosition(ctx, subcolumn.ColumnID)
		if err != nil {
			return 0, err
		}
		subcolumn.Position = pos
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_subcolumns(column_id, name, position, is_active, created_at, updated_at)
		VALUES(?,?,?,?,?,?)`,
		subcolumn.ColumnID, subcolumn.Name, subcolumn.Position, boolToInt(subcolumn.IsActive), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	subcolumn.ID = id
	subcolumn.CreatedAt = now
	subcolumn.UpdatedAt = now
	return id, nil
}

func (s *SQLStore) UpdateSubColumn(ctx context.Context, subcolumn *tasks.SubColumn) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_subcolumns SET name=?, position=?, is_active=?, updated_at=? WHERE id=?`,
		subcolumn.Name, subcolumn.Position, boolToInt(subcolumn.IsActive), now, subcolumn.ID)
	if err != nil {
		return err
	}
	subcolumn.UpdatedAt = now
	return nil
}

func (s *SQLStore) MoveSubColumn(ctx context.Context, subcolumnID int64, position int) (*tasks.SubColumn, error) {
	var subcolumn *tasks.SubColumn
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		subcolumn, err = s.getSubColumnTx(ctx, tx, subcolumnID)
		if err != nil {
			return err
		}
		if subcolumn == nil {
			return sql.ErrNoRows
		}
		if position <= 0 {
			row := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_subcolumns WHERE column_id=?`, subcolumn.ColumnID)
			var max int
			if err := row.Scan(&max); err != nil {
				return err
			}
			position = max
		}
		if position == subcolumn.Position {
			return nil
		}
		if position > subcolumn.Position {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_subcolumns SET position=position-1
				WHERE column_id=? AND position>? AND position<=?`, subcolumn.ColumnID, subcolumn.Position, position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_subcolumns SET position=position+1
				WHERE column_id=? AND position>=? AND position<?`, subcolumn.ColumnID, position, subcolumn.Position); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE task_subcolumns SET position=?, updated_at=? WHERE id=?`, position, now, subcolumnID); err != nil {
			return err
		}
		subcolumn.Position = position
		subcolumn.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return subcolumn, nil
}

func (s *SQLStore) DeleteSubColumn(ctx context.Context, subcolumnID int64) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_subcolumns SET is_active=0, updated_at=? WHERE id=?`, time.Now().UTC(), subcolumnID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE tasks SET subcolumn_id=NULL, updated_at=? WHERE subcolumn_id=?`, time.Now().UTC(), subcolumnID); err != nil {
			return err
		}
		return nil
	})
}

func (s *SQLStore) GetSubColumn(ctx context.Context, subcolumnID int64) (*tasks.SubColumn, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, column_id, name, position, is_active, created_at, updated_at
		FROM task_subcolumns WHERE id=?`, subcolumnID)
	var sc tasks.SubColumn
	var active int
	if err := row.Scan(&sc.ID, &sc.ColumnID, &sc.Name, &sc.Position, &active, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	sc.IsActive = active == 1
	return &sc, nil
}

func (s *SQLStore) ListSubColumns(ctx context.Context, columnID int64, includeInactive bool) ([]tasks.SubColumn, error) {
	query := `
		SELECT id, column_id, name, position, is_active, created_at, updated_at
		FROM task_subcolumns WHERE column_id=?`
	args := []any{columnID}
	if !includeInactive {
		query += " AND is_active=1"
	}
	query += " ORDER BY position ASC, id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.SubColumn
	for rows.Next() {
		var sc tasks.SubColumn
		var active int
		if err := rows.Scan(&sc.ID, &sc.ColumnID, &sc.Name, &sc.Position, &active, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		sc.IsActive = active == 1
		res = append(res, sc)
	}
	return res, rows.Err()
}

func (s *SQLStore) ListSubColumnsByBoard(ctx context.Context, boardID int64, includeInactive bool) ([]tasks.SubColumn, error) {
	query := `
		SELECT sc.id, sc.column_id, sc.name, sc.position, sc.is_active, sc.created_at, sc.updated_at
		FROM task_subcolumns sc
		JOIN task_columns c ON sc.column_id=c.id
		WHERE c.board_id=?`
	args := []any{boardID}
	if !includeInactive {
		query += " AND sc.is_active=1"
	}
	query += " ORDER BY c.position ASC, sc.position ASC, sc.id ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.SubColumn
	for rows.Next() {
		var sc tasks.SubColumn
		var active int
		if err := rows.Scan(&sc.ID, &sc.ColumnID, &sc.Name, &sc.Position, &active, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		sc.IsActive = active == 1
		res = append(res, sc)
	}
	return res, rows.Err()
}

func (s *SQLStore) NextSubColumnPosition(ctx context.Context, columnID int64) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_subcolumns WHERE column_id=?`, columnID)
	var max int
	if err := row.Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func (s *SQLStore) CountTasksInSubColumn(ctx context.Context, subcolumnID int64) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM tasks WHERE subcolumn_id=? AND is_archived=0`, subcolumnID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLStore) getSubColumnTx(ctx context.Context, tx *sql.Tx, subcolumnID int64) (*tasks.SubColumn, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, column_id, name, position, is_active, created_at, updated_at
		FROM task_subcolumns WHERE id=?`, subcolumnID)
	var sc tasks.SubColumn
	var active int
	if err := row.Scan(&sc.ID, &sc.ColumnID, &sc.Name, &sc.Position, &active, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	sc.IsActive = active == 1
	return &sc, nil
}
