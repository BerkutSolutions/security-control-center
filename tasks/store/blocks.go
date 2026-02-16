package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) CreateTaskBlock(ctx context.Context, block *tasks.TaskBlock) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_blocks(task_id, block_type, reason, blocker_task_id, created_by, created_at, is_active)
		VALUES(?,?,?,?,?,?,?)`,
		block.TaskID, block.BlockType, block.Reason, nullableID(block.BlockerTaskID), nullableID(block.CreatedBy), now, 1)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	block.ID = id
	block.CreatedAt = now
	block.IsActive = true
	return id, nil
}

func (s *SQLStore) ResolveTaskBlock(ctx context.Context, taskID int64, blockID int64, resolvedBy int64) (*tasks.TaskBlock, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE task_blocks
		SET is_active=0, resolved_by=?, resolved_at=?
		WHERE id=? AND task_id=? AND is_active=1`,
		resolvedBy, now, blockID, taskID)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, nil
	}
	block, err := s.getTaskBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}
	if block != nil {
		block.ResolvedBy = &resolvedBy
		block.ResolvedAt = &now
		block.IsActive = false
	}
	return block, nil
}

func (s *SQLStore) ListTaskBlocks(ctx context.Context, taskID int64) ([]tasks.TaskBlock, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, block_type, reason, blocker_task_id, created_by, created_at, resolved_by, resolved_at, is_active
		FROM task_blocks WHERE task_id=?
		ORDER BY created_at DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskBlock
	for rows.Next() {
		block, err := scanTaskBlockRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, block)
	}
	return res, rows.Err()
}

func (s *SQLStore) ListActiveTaskBlocksForTasks(ctx context.Context, taskIDs []int64) (map[int64][]tasks.TaskBlock, error) {
	res := map[int64][]tasks.TaskBlock{}
	if len(taskIDs) == 0 {
		return res, nil
	}
	args := toAny(taskIDs)
	query := `
		SELECT id, task_id, block_type, reason, blocker_task_id, created_by, created_at, resolved_by, resolved_at, is_active
		FROM task_blocks
		WHERE is_active=1 AND task_id IN (` + placeholders(len(args)) + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		block, err := scanTaskBlockRow(rows)
		if err != nil {
			return nil, err
		}
		res[block.TaskID] = append(res[block.TaskID], block)
	}
	return res, rows.Err()
}

func (s *SQLStore) ListActiveBlocksByBlocker(ctx context.Context, blockerTaskID int64) ([]tasks.TaskBlock, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, block_type, reason, blocker_task_id, created_by, created_at, resolved_by, resolved_at, is_active
		FROM task_blocks
		WHERE is_active=1 AND blocker_task_id=?
		ORDER BY created_at DESC`, blockerTaskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskBlock
	for rows.Next() {
		block, err := scanTaskBlockRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, block)
	}
	return res, rows.Err()
}

func (s *SQLStore) ResolveTaskBlocksByBlocker(ctx context.Context, blockerTaskID int64, resolvedBy int64) ([]tasks.TaskBlock, error) {
	var blocks []tasks.TaskBlock
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, task_id, block_type, reason, blocker_task_id, created_by, created_at, resolved_by, resolved_at, is_active
			FROM task_blocks
			WHERE is_active=1 AND blocker_task_id=?`, blockerTaskID)
		if err != nil {
			return err
		}
		for rows.Next() {
			block, err := scanTaskBlockRow(rows)
			if err != nil {
				rows.Close()
				return err
			}
			blocks = append(blocks, block)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(blocks) == 0 {
			return nil
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_blocks
			SET is_active=0, resolved_by=?, resolved_at=?
			WHERE is_active=1 AND blocker_task_id=?`, resolvedBy, now, blockerTaskID); err != nil {
			return err
		}
		for i := range blocks {
			blocks[i].ResolvedBy = &resolvedBy
			blocks[i].ResolvedAt = &now
			blocks[i].IsActive = false
		}
		return nil
	})
	return blocks, err
}

func (s *SQLStore) getTaskBlock(ctx context.Context, blockID int64) (*tasks.TaskBlock, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, block_type, reason, blocker_task_id, created_by, created_at, resolved_by, resolved_at, is_active
		FROM task_blocks WHERE id=?`, blockID)
	var block tasks.TaskBlock
	if err := scanTaskBlock(row, &block); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &block, nil
}

func scanTaskBlock(row *sql.Row, block *tasks.TaskBlock) error {
	var reason sql.NullString
	var blockerID sql.NullInt64
	var createdBy sql.NullInt64
	var resolvedBy sql.NullInt64
	var resolvedAt sql.NullTime
	var active int
	if err := row.Scan(
		&block.ID, &block.TaskID, &block.BlockType, &reason, &blockerID,
		&createdBy, &block.CreatedAt, &resolvedBy, &resolvedAt, &active,
	); err != nil {
		return err
	}
	if reason.Valid {
		block.Reason = &reason.String
	}
	if blockerID.Valid {
		block.BlockerTaskID = &blockerID.Int64
	}
	if createdBy.Valid {
		block.CreatedBy = &createdBy.Int64
	}
	if resolvedBy.Valid {
		block.ResolvedBy = &resolvedBy.Int64
	}
	if resolvedAt.Valid {
		block.ResolvedAt = &resolvedAt.Time
	}
	block.IsActive = active == 1
	return nil
}

func scanTaskBlockRow(rows *sql.Rows) (tasks.TaskBlock, error) {
	var block tasks.TaskBlock
	var reason sql.NullString
	var blockerID sql.NullInt64
	var createdBy sql.NullInt64
	var resolvedBy sql.NullInt64
	var resolvedAt sql.NullTime
	var active int
	if err := rows.Scan(
		&block.ID, &block.TaskID, &block.BlockType, &reason, &blockerID,
		&createdBy, &block.CreatedAt, &resolvedBy, &resolvedAt, &active,
	); err != nil {
		return block, err
	}
	if reason.Valid {
		block.Reason = &reason.String
	}
	if blockerID.Valid {
		block.BlockerTaskID = &blockerID.Int64
	}
	if createdBy.Valid {
		block.CreatedBy = &createdBy.Int64
	}
	if resolvedBy.Valid {
		block.ResolvedBy = &resolvedBy.Int64
	}
	if resolvedAt.Valid {
		block.ResolvedAt = &resolvedAt.Time
	}
	block.IsActive = active == 1
	return block, nil
}

