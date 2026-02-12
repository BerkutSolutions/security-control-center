package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) CreateBoard(ctx context.Context, board *tasks.Board, acl []tasks.ACLRule) (int64, error) {
	if board.SpaceID <= 0 {
		return 0, tasks.ErrInvalidInput
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM task_spaces WHERE id=? AND is_active=1`, board.SpaceID).Scan(&exists); err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, tasks.ErrInvalidInput
	}
	if board.Position <= 0 {
		pos, err := s.NextBoardPosition(ctx, board.SpaceID)
		if err != nil {
			return 0, err
		}
		board.Position = pos
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_boards(space_id, organization_id, name, description, default_template_id, position, created_by, created_at, updated_at, is_active)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		board.SpaceID, strings.TrimSpace(board.OrganizationID), board.Name, board.Description, nullableID(board.DefaultTemplateID), board.Position, nullableID(board.CreatedBy), now, now, boolToInt(board.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	board.ID = id
	board.CreatedAt = now
	board.UpdatedAt = now
	if len(acl) == 0 {
		return id, nil
	}
	if err := s.SetBoardACL(ctx, id, acl); err != nil {
		return id, err
	}
	return id, nil
}

func (s *SQLiteStore) UpdateBoard(ctx context.Context, board *tasks.Board) error {
	if board.SpaceID <= 0 {
		return tasks.ErrInvalidInput
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM task_spaces WHERE id=?`, board.SpaceID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return tasks.ErrInvalidInput
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_boards SET space_id=?, organization_id=?, name=?, description=?, default_template_id=?, position=?, is_active=?, updated_at=?
		WHERE id=?`,
		board.SpaceID, strings.TrimSpace(board.OrganizationID), board.Name, board.Description, nullableID(board.DefaultTemplateID), board.Position, boolToInt(board.IsActive), time.Now().UTC(), board.ID)
	return err
}

func (s *SQLiteStore) MoveBoard(ctx context.Context, boardID int64, position int) (*tasks.Board, error) {
	var board *tasks.Board
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		board, err = s.getBoardTx(ctx, tx, boardID)
		if err != nil {
			return err
		}
		if board == nil {
			return sql.ErrNoRows
		}
		if position <= 0 {
			row := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_boards WHERE space_id=?`, board.SpaceID)
			var max int
			if err := row.Scan(&max); err != nil {
				return err
			}
			position = max
		}
		if position == board.Position {
			return nil
		}
		if position > board.Position {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_boards SET position=position-1
				WHERE space_id=? AND position>? AND position<=?`,
				board.SpaceID, board.Position, position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_boards SET position=position+1
				WHERE space_id=? AND position>=? AND position<?`,
				board.SpaceID, position, board.Position); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `UPDATE task_boards SET position=?, updated_at=? WHERE id=?`, position, now, boardID); err != nil {
			return err
		}
		board.Position = position
		board.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return board, nil
}

func (s *SQLiteStore) DeleteBoard(ctx context.Context, boardID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_boards SET is_active=0, updated_at=? WHERE id=?`, time.Now().UTC(), boardID)
	return err
}

func (s *SQLiteStore) GetBoard(ctx context.Context, boardID int64) (*tasks.Board, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, space_id, organization_id, name, description, default_template_id, position, created_by, created_at, updated_at, is_active
		FROM task_boards WHERE id=?`, boardID)
	var b tasks.Board
	var createdBy sql.NullInt64
	var defaultTemplateID sql.NullInt64
	var active int
	if err := row.Scan(&b.ID, &b.SpaceID, &b.OrganizationID, &b.Name, &b.Description, &defaultTemplateID, &b.Position, &createdBy, &b.CreatedAt, &b.UpdatedAt, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		b.CreatedBy = &createdBy.Int64
	}
	if defaultTemplateID.Valid {
		b.DefaultTemplateID = &defaultTemplateID.Int64
	}
	b.IsActive = active == 1
	return &b, nil
}

func (s *SQLiteStore) ListBoards(ctx context.Context, filter tasks.BoardFilter) ([]tasks.Board, error) {
	query := `
		SELECT id, space_id, organization_id, name, description, default_template_id, position, created_by, created_at, updated_at, is_active
		FROM task_boards`
	clauses := []string{}
	args := []any{}
	if filter.SpaceID > 0 {
		clauses = append(clauses, "space_id=?")
		args = append(args, filter.SpaceID)
	}
	if !filter.IncludeInactive {
		clauses = append(clauses, "is_active=1")
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY position ASC, updated_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Board
	for rows.Next() {
		var b tasks.Board
		var createdBy sql.NullInt64
		var defaultTemplateID sql.NullInt64
		var active int
		if err := rows.Scan(&b.ID, &b.SpaceID, &b.OrganizationID, &b.Name, &b.Description, &defaultTemplateID, &b.Position, &createdBy, &b.CreatedAt, &b.UpdatedAt, &active); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			b.CreatedBy = &createdBy.Int64
		}
		if defaultTemplateID.Valid {
			b.DefaultTemplateID = &defaultTemplateID.Int64
		}
		b.IsActive = active == 1
		res = append(res, b)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) SetBoardACL(ctx context.Context, boardID int64, acl []tasks.ACLRule) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_board_acl WHERE board_id=?`, boardID); err != nil {
			return err
		}
		for _, a := range acl {
			if _, err := tx.ExecContext(ctx, `INSERT INTO task_board_acl(board_id, subject_type, subject_id, permission) VALUES(?,?,?,?)`,
				boardID, strings.ToLower(a.SubjectType), a.SubjectID, strings.ToLower(a.Permission)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLiteStore) GetBoardACL(ctx context.Context, boardID int64) ([]tasks.ACLRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT subject_type, subject_id, permission FROM task_board_acl WHERE board_id=?`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.ACLRule
	for rows.Next() {
		var a tasks.ACLRule
		if err := rows.Scan(&a.SubjectType, &a.SubjectID, &a.Permission); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) NextBoardPosition(ctx context.Context, spaceID int64) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM task_boards WHERE space_id=?`, spaceID)
	var max int
	if err := row.Scan(&max); err != nil {
		return 0, err
	}
	return max + 1, nil
}

func (s *SQLiteStore) getBoardTx(ctx context.Context, tx *sql.Tx, boardID int64) (*tasks.Board, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, space_id, organization_id, name, description, default_template_id, position, created_by, created_at, updated_at, is_active
		FROM task_boards WHERE id=?`, boardID)
	var b tasks.Board
	var createdBy sql.NullInt64
	var defaultTemplateID sql.NullInt64
	var active int
	if err := row.Scan(&b.ID, &b.SpaceID, &b.OrganizationID, &b.Name, &b.Description, &defaultTemplateID, &b.Position, &createdBy, &b.CreatedAt, &b.UpdatedAt, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		b.CreatedBy = &createdBy.Int64
	}
	if defaultTemplateID.Valid {
		b.DefaultTemplateID = &defaultTemplateID.Int64
	}
	b.IsActive = active == 1
	return &b, nil
}
