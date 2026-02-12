package store

import (
	"context"
	"database/sql"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) SetTaskAssignments(ctx context.Context, taskID int64, userIDs []int64, assignedBy int64) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_assignments WHERE task_id=?`, taskID); err != nil {
			return err
		}
		now := time.Now().UTC()
		seen := map[int64]struct{}{}
		for _, uid := range userIDs {
			if uid == 0 {
				continue
			}
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task_assignments(task_id, user_id, assigned_at, assigned_by)
				VALUES(?,?,?,?)`, taskID, uid, now, assignedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLiteStore) ListTaskAssignments(ctx context.Context, taskID int64) ([]tasks.Assignment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, user_id, assigned_at, assigned_by
		FROM task_assignments WHERE task_id=? ORDER BY assigned_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Assignment
	for rows.Next() {
		var a tasks.Assignment
		var assignedBy sql.NullInt64
		if err := rows.Scan(&a.TaskID, &a.UserID, &a.AssignedAt, &assignedBy); err != nil {
			return nil, err
		}
		if assignedBy.Valid {
			a.AssignedBy = &assignedBy.Int64
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) ListTaskAssignmentsForTasks(ctx context.Context, taskIDs []int64) (map[int64][]tasks.Assignment, error) {
	out := map[int64][]tasks.Assignment{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, user_id, assigned_at, assigned_by
		FROM task_assignments WHERE task_id IN (`+placeholders(len(taskIDs))+`) ORDER BY assigned_at ASC`, toAny(taskIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a tasks.Assignment
		var assignedBy sql.NullInt64
		if err := rows.Scan(&a.TaskID, &a.UserID, &a.AssignedAt, &assignedBy); err != nil {
			return nil, err
		}
		if assignedBy.Valid {
			a.AssignedBy = &assignedBy.Int64
		}
		out[a.TaskID] = append(out[a.TaskID], a)
	}
	return out, rows.Err()
}
