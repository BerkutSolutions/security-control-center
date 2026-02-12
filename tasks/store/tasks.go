package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) CreateTask(ctx context.Context, task *tasks.Task, assignments []int64) (int64, error) {
	var taskID int64
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		id, err := s.createTaskTx(ctx, tx, task, assignments)
		if err != nil {
			return err
		}
		taskID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return taskID, nil
}

func (s *SQLiteStore) UpdateTask(ctx context.Context, task *tasks.Task) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET title=?, description=?, result=?, external_link=?, business_customer=?, size_estimate=?, priority=?, checklist=?, due_date=?, updated_at=? WHERE id=?`,
		task.Title, task.Description, task.Result, task.ExternalLink, task.BusinessCustomer, nullableInt(task.SizeEstimate), task.Priority, marshalJSON(task.Checklist), nullableTime(task.DueDate), time.Now().UTC(), task.ID)
	return err
}

func (s *SQLiteStore) MoveTask(ctx context.Context, taskID int64, columnID int64, subcolumnID *int64, position int) (*tasks.Task, error) {
	var task *tasks.Task
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		task, err = s.getTaskTx(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return sql.ErrNoRows
		}
		var targetBoardID int64
		var targetName string
		if err := tx.QueryRowContext(ctx, `SELECT board_id, name FROM task_columns WHERE id=?`, columnID).Scan(&targetBoardID, &targetName); err != nil {
			return err
		}
		if targetBoardID != task.BoardID {
			return fmt.Errorf("column board mismatch")
		}
		if subcolumnID != nil {
			var ownerColumnID int64
			if err := tx.QueryRowContext(ctx, `SELECT column_id FROM task_subcolumns WHERE id=? AND is_active=1`, *subcolumnID).Scan(&ownerColumnID); err != nil {
				return err
			}
			if ownerColumnID != columnID {
				return fmt.Errorf("subcolumn column mismatch")
			}
		}
		sameSubcolumn := (task.SubColumnID == nil && subcolumnID == nil) ||
			(task.SubColumnID != nil && subcolumnID != nil && *task.SubColumnID == *subcolumnID)
		if position <= 0 {
			var row *sql.Row
			if subcolumnID != nil {
				row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id=?`, columnID, *subcolumnID)
			} else {
				row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id IS NULL`, columnID)
			}
			var max int
			if err := row.Scan(&max); err != nil {
				return err
			}
			position = max + 1
		}
		if task.ColumnID == columnID && sameSubcolumn {
			if position != task.Position {
				if position > task.Position {
					if task.SubColumnID != nil {
						_, err = tx.ExecContext(ctx, `
							UPDATE tasks SET position=position-1
							WHERE column_id=? AND subcolumn_id=? AND position>? AND position<=?`, columnID, *task.SubColumnID, task.Position, position)
					} else {
						_, err = tx.ExecContext(ctx, `
							UPDATE tasks SET position=position-1
							WHERE column_id=? AND subcolumn_id IS NULL AND position>? AND position<=?`, columnID, task.Position, position)
					}
				} else {
					if task.SubColumnID != nil {
						_, err = tx.ExecContext(ctx, `
							UPDATE tasks SET position=position+1
							WHERE column_id=? AND subcolumn_id=? AND position>=? AND position<?`, columnID, *task.SubColumnID, position, task.Position)
					} else {
						_, err = tx.ExecContext(ctx, `
							UPDATE tasks SET position=position+1
							WHERE column_id=? AND subcolumn_id IS NULL AND position>=? AND position<?`, columnID, position, task.Position)
					}
				}
				if err != nil {
					return err
				}
			}
		} else {
			if task.SubColumnID != nil {
				if _, err := tx.ExecContext(ctx, `
					UPDATE tasks SET position=position-1
					WHERE column_id=? AND subcolumn_id=? AND position>?`, task.ColumnID, *task.SubColumnID, task.Position); err != nil {
					return err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `
					UPDATE tasks SET position=position-1
					WHERE column_id=? AND subcolumn_id IS NULL AND position>?`, task.ColumnID, task.Position); err != nil {
					return err
				}
			}
			if subcolumnID != nil {
				if _, err := tx.ExecContext(ctx, `
					UPDATE tasks SET position=position+1
					WHERE column_id=? AND subcolumn_id=? AND position>=?`, columnID, *subcolumnID, position); err != nil {
					return err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `
					UPDATE tasks SET position=position+1
					WHERE column_id=? AND subcolumn_id IS NULL AND position>=?`, columnID, position); err != nil {
					return err
				}
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			UPDATE tasks SET column_id=?, subcolumn_id=?, status=?, position=?, updated_at=? WHERE id=?`,
			columnID, nullableID(subcolumnID), targetName, position, now, taskID); err != nil {
			return err
		}
		task.ColumnID = columnID
		task.SubColumnID = subcolumnID
		task.Status = targetName
		task.Position = position
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *SQLiteStore) RelocateTask(ctx context.Context, taskID int64, boardID int64, columnID int64, subcolumnID *int64, position int) (*tasks.Task, error) {
	var task *tasks.Task
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		task, err = s.getTaskTx(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return sql.ErrNoRows
		}
		var targetBoardID int64
		var targetName string
		if err := tx.QueryRowContext(ctx, `SELECT board_id, name FROM task_columns WHERE id=?`, columnID).Scan(&targetBoardID, &targetName); err != nil {
			return err
		}
		if boardID == 0 {
			boardID = targetBoardID
		}
		if targetBoardID != boardID {
			return fmt.Errorf("column board mismatch")
		}
		if subcolumnID != nil {
			var ownerColumnID int64
			if err := tx.QueryRowContext(ctx, `SELECT column_id FROM task_subcolumns WHERE id=? AND is_active=1`, *subcolumnID).Scan(&ownerColumnID); err != nil {
				return err
			}
			if ownerColumnID != columnID {
				return fmt.Errorf("subcolumn column mismatch")
			}
		}
		if position <= 0 {
			var row *sql.Row
			if subcolumnID != nil {
				row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id=?`, columnID, *subcolumnID)
			} else {
				row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id IS NULL`, columnID)
			}
			var max int
			if err := row.Scan(&max); err != nil {
				return err
			}
			position = max + 1
		}
		if task.SubColumnID != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position-1
				WHERE column_id=? AND subcolumn_id=? AND position>?`, task.ColumnID, *task.SubColumnID, task.Position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position-1
				WHERE column_id=? AND subcolumn_id IS NULL AND position>?`, task.ColumnID, task.Position); err != nil {
				return err
			}
		}
		if subcolumnID != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position+1
				WHERE column_id=? AND subcolumn_id=? AND position>=?`, columnID, *subcolumnID, position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position+1
				WHERE column_id=? AND subcolumn_id IS NULL AND position>=?`, columnID, position); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			UPDATE tasks SET board_id=?, column_id=?, subcolumn_id=?, status=?, position=?, updated_at=? WHERE id=?`,
			boardID, columnID, nullableID(subcolumnID), targetName, position, now, taskID); err != nil {
			return err
		}
		task.BoardID = boardID
		task.ColumnID = columnID
		task.SubColumnID = subcolumnID
		task.Status = targetName
		task.Position = position
		task.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *SQLiteStore) CloseTask(ctx context.Context, taskID int64, userID int64) (*tasks.Task, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET closed_at=?, updated_at=? WHERE id=? AND closed_at IS NULL`, now, now, taskID)
	if err != nil {
		return nil, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return nil, tasks.ErrConflict
	}
	return s.GetTask(ctx, taskID)
}

func (s *SQLiteStore) ArchiveTask(ctx context.Context, taskID int64, userID int64) (*tasks.Task, error) {
	var archived *tasks.Task
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		archived, err = s.archiveTaskTx(ctx, tx, taskID, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return archived, nil
}

func (s *SQLiteStore) ArchiveTasksByColumn(ctx context.Context, columnID int64, userID int64) (int, error) {
	var archivedCount int
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM tasks WHERE column_id=? AND is_archived=0 ORDER BY position ASC, id ASC`, columnID)
		if err != nil {
			return err
		}
		defer rows.Close()
		var taskIDs []int64
		for rows.Next() {
			var taskID int64
			if err := rows.Scan(&taskID); err != nil {
				return err
			}
			taskIDs = append(taskIDs, taskID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, taskID := range taskIDs {
			if _, err := s.archiveTaskTx(ctx, tx, taskID, userID); err != nil {
				if errors.Is(err, tasks.ErrConflict) {
					continue
				}
				return err
			}
			archivedCount++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return archivedCount, nil
}

func (s *SQLiteStore) RestoreTask(ctx context.Context, taskID int64, boardID int64, columnID int64, subcolumnID *int64, userID int64) (*tasks.Task, error) {
	var restored *tasks.Task
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		task, err := s.getTaskTx(ctx, tx, taskID)
		if err != nil {
			return err
		}
		if task == nil {
			return tasks.ErrNotFound
		}
		if !task.IsArchived {
			return tasks.ErrConflict
		}
		row := tx.QueryRowContext(ctx, `
			SELECT archived_board_id, archived_column_id, archived_subcolumn_id, original_position
			FROM task_archive_entries WHERE task_id=?`, taskID)
		var archivedBoardID int64
		var archivedColumnID int64
		var archivedSubColumnID sql.NullInt64
		var originalPosition int
		if err := row.Scan(&archivedBoardID, &archivedColumnID, &archivedSubColumnID, &originalPosition); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				archivedBoardID = task.BoardID
				archivedColumnID = task.ColumnID
				originalPosition = task.Position
			} else {
				return err
			}
		}
		targetBoardID := boardID
		if targetBoardID <= 0 {
			targetBoardID = archivedBoardID
		}
		targetColumnID := columnID
		if targetColumnID <= 0 {
			targetColumnID = archivedColumnID
		}
		targetSubColumnID := subcolumnID
		if targetSubColumnID == nil && archivedSubColumnID.Valid {
			targetSubColumnID = &archivedSubColumnID.Int64
		}
		if targetBoardID <= 0 || targetColumnID <= 0 {
			return tasks.ErrInvalidInput
		}

		var boardActive int
		if err := tx.QueryRowContext(ctx, `SELECT is_active FROM task_boards WHERE id=?`, targetBoardID).Scan(&boardActive); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return tasks.ErrNotFound
			}
			return err
		}
		if boardActive != 1 && boardID <= 0 {
			return tasks.ErrInvalidInput
		}
		var targetColumnBoardID int64
		var targetColumnName string
		var targetColumnActive int
		if err := tx.QueryRowContext(ctx, `SELECT board_id, name, is_active FROM task_columns WHERE id=?`, targetColumnID).Scan(&targetColumnBoardID, &targetColumnName, &targetColumnActive); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return tasks.ErrNotFound
			}
			return err
		}
		if targetColumnBoardID != targetBoardID {
			return tasks.ErrInvalidInput
		}
		if targetColumnActive != 1 && columnID <= 0 {
			return tasks.ErrInvalidInput
		}
		if targetSubColumnID != nil {
			var ownerColumnID int64
			var active int
			if err := tx.QueryRowContext(ctx, `SELECT column_id, is_active FROM task_subcolumns WHERE id=?`, *targetSubColumnID).Scan(&ownerColumnID, &active); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return tasks.ErrNotFound
				}
				return err
			}
			if ownerColumnID != targetColumnID {
				return tasks.ErrInvalidInput
			}
			if active != 1 {
				targetSubColumnID = nil
			}
		}
		position := originalPosition
		if position <= 0 {
			position = 1
		}
		if targetSubColumnID != nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position+1
				WHERE is_archived=0 AND column_id=? AND subcolumn_id=? AND position>=?`,
				targetColumnID, *targetSubColumnID, position); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks SET position=position+1
				WHERE is_archived=0 AND column_id=? AND subcolumn_id IS NULL AND position>=?`,
				targetColumnID, position); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET board_id=?, column_id=?, subcolumn_id=?, status=?, position=?, is_archived=0, updated_at=?
			WHERE id=?`,
			targetBoardID, targetColumnID, nullableID(targetSubColumnID), targetColumnName, position, now, taskID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_archive_entries
			SET restored_at=?, restored_by=?
			WHERE task_id=?`,
			now, userID, taskID); err != nil {
			return err
		}
		restored, err = s.getTaskTx(ctx, tx, taskID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return restored, nil
}

func (s *SQLiteStore) ListArchivedTasks(ctx context.Context, filter tasks.TaskFilter) ([]tasks.TaskArchiveEntry, error) {
	base := "tasks t JOIN task_archive_entries a ON a.task_id=t.id"
	clauses := []string{"t.is_archived=1"}
	args := []any{}
	selectPrefix := `
		SELECT
			t.id, t.board_id, t.column_id, t.subcolumn_id, t.title, t.description, t.result, t.external_link, t.business_customer, t.size_estimate,
			t.status, t.priority, t.template_id, t.recurring_rule_id, t.checklist, t.created_by, t.due_date, t.created_at, t.updated_at, t.closed_at,
			t.is_archived, t.position,
			a.archived_at, a.archived_by, a.archived_board_id, a.archived_column_id, a.archived_subcolumn_id, a.original_position, a.restored_at
	`
	if filter.SpaceID > 0 {
		base += " JOIN task_boards b ON b.id=t.board_id"
		clauses = append(clauses, "b.space_id=?")
		args = append(args, filter.SpaceID)
	}
	if filter.BoardID > 0 {
		clauses = append(clauses, "t.board_id=?")
		args = append(args, filter.BoardID)
	}
	if filter.ColumnID > 0 {
		clauses = append(clauses, "t.column_id=?")
		args = append(args, filter.ColumnID)
	}
	if filter.SubColumnID > 0 {
		clauses = append(clauses, "t.subcolumn_id=?")
		args = append(args, filter.SubColumnID)
	}
	if strings.TrimSpace(filter.Search) != "" {
		query := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		clauses = append(clauses, "(lower(t.title) LIKE ? OR lower(t.description) LIKE ? OR lower(t.result) LIKE ?)")
		args = append(args, query, query, query)
	}
	query := fmt.Sprintf("%s FROM %s WHERE %s ORDER BY a.archived_at DESC", selectPrefix, base, strings.Join(clauses, " AND "))
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]tasks.TaskArchiveEntry, 0)
	for rows.Next() {
		entry, err := scanArchivedTaskRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) DeleteTask(ctx context.Context, taskID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE id=?`, taskID)
	return err
}

func (s *SQLiteStore) GetTask(ctx context.Context, taskID int64) (*tasks.Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, board_id, column_id, subcolumn_id, title, description, result, external_link, business_customer, size_estimate, status, priority, template_id, recurring_rule_id, checklist, created_by, due_date, created_at, updated_at, closed_at, is_archived, position
		FROM tasks WHERE id=?`, taskID)
	return s.scanTask(row)
}

func (s *SQLiteStore) ListTasks(ctx context.Context, filter tasks.TaskFilter) ([]tasks.Task, error) {
	clauses := []string{}
	args := []any{}
	base := "tasks"
	selectPrefix := "SELECT id, board_id, column_id, subcolumn_id, title, description, result, external_link, business_customer, size_estimate, status, priority, template_id, recurring_rule_id, checklist, created_by, due_date, created_at, updated_at, closed_at, is_archived, position"
	if filter.SpaceID > 0 {
		base = "tasks t JOIN task_boards b ON t.board_id=b.id"
		selectPrefix = "SELECT t.id, t.board_id, t.column_id, t.subcolumn_id, t.title, t.description, t.result, t.external_link, t.business_customer, t.size_estimate, t.status, t.priority, t.template_id, t.recurring_rule_id, t.checklist, t.created_by, t.due_date, t.created_at, t.updated_at, t.closed_at, t.is_archived, t.position"
		clauses = append(clauses, "b.space_id=?")
		args = append(args, filter.SpaceID)
	}
	if filter.BoardID > 0 {
		clauses = append(clauses, "board_id=?")
		args = append(args, filter.BoardID)
	}
	if filter.ColumnID > 0 {
		clauses = append(clauses, "column_id=?")
		args = append(args, filter.ColumnID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status=?")
		args = append(args, filter.Status)
	}
	if strings.TrimSpace(filter.Search) != "" {
		query := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		clauses = append(clauses, "(lower(title) LIKE ? OR lower(description) LIKE ? OR lower(result) LIKE ?)")
		args = append(args, query, query)
		args = append(args, query)
	}
	if !filter.IncludeArchived {
		clauses = append(clauses, "is_archived=0")
	}
	if filter.AssignedUserID > 0 {
		clauses = append(clauses, "id IN (SELECT task_id FROM task_assignments WHERE user_id=?)")
		args = append(args, filter.AssignedUserID)
	}
	if filter.MineUserID > 0 {
		clauses = append(clauses, "id IN (SELECT task_id FROM task_assignments WHERE user_id=?)")
		args = append(args, filter.MineUserID)
	}
	query := fmt.Sprintf("%s FROM %s", selectPrefix, base)
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filter.Offset)
		}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Task
	for rows.Next() {
		task, err := s.scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, task)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) CountTasksByBoard(ctx context.Context, boardIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int)
	if len(boardIDs) == 0 {
		return result, nil
	}
	query := fmt.Sprintf(
		`SELECT board_id, COUNT(*) FROM tasks WHERE is_archived=0 AND board_id IN (%s) GROUP BY board_id`,
		placeholders(len(boardIDs)),
	)
	rows, err := s.db.QueryContext(ctx, query, toAny(boardIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var boardID int64
		var count int
		if err := rows.Scan(&boardID, &count); err != nil {
			return nil, err
		}
		result[boardID] = count
	}
	return result, rows.Err()
}

func (s *SQLiteStore) scanTask(row *sql.Row) (*tasks.Task, error) {
	var t tasks.Task
	var createdBy sql.NullInt64
	var templateID sql.NullInt64
	var recurringID sql.NullInt64
	var checklistRaw sql.NullString
	var due sql.NullTime
	var closed sql.NullTime
	var archived int
	var sizeEstimate sql.NullInt64
	var subcolumnID sql.NullInt64
	if err := row.Scan(&t.ID, &t.BoardID, &t.ColumnID, &subcolumnID, &t.Title, &t.Description, &t.Result, &t.ExternalLink, &t.BusinessCustomer, &sizeEstimate, &t.Status, &t.Priority, &templateID, &recurringID, &checklistRaw, &createdBy, &due, &t.CreatedAt, &t.UpdatedAt, &closed, &archived, &t.Position); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if templateID.Valid {
		t.TemplateID = &templateID.Int64
	}
	if recurringID.Valid {
		t.RecurringRuleID = &recurringID.Int64
	}
	if subcolumnID.Valid {
		t.SubColumnID = &subcolumnID.Int64
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &t.Checklist)
	}
	if sizeEstimate.Valid {
		val := int(sizeEstimate.Int64)
		t.SizeEstimate = &val
	}
	if createdBy.Valid {
		t.CreatedBy = &createdBy.Int64
	}
	if due.Valid {
		t.DueDate = &due.Time
	}
	if closed.Valid {
		t.ClosedAt = &closed.Time
	}
	t.IsArchived = archived == 1
	return &t, nil
}

func (s *SQLiteStore) scanTaskRow(rows *sql.Rows) (tasks.Task, error) {
	var t tasks.Task
	var createdBy sql.NullInt64
	var templateID sql.NullInt64
	var recurringID sql.NullInt64
	var checklistRaw sql.NullString
	var due sql.NullTime
	var closed sql.NullTime
	var archived int
	var sizeEstimate sql.NullInt64
	var subcolumnID sql.NullInt64
	if err := rows.Scan(&t.ID, &t.BoardID, &t.ColumnID, &subcolumnID, &t.Title, &t.Description, &t.Result, &t.ExternalLink, &t.BusinessCustomer, &sizeEstimate, &t.Status, &t.Priority, &templateID, &recurringID, &checklistRaw, &createdBy, &due, &t.CreatedAt, &t.UpdatedAt, &closed, &archived, &t.Position); err != nil {
		return t, err
	}
	if templateID.Valid {
		t.TemplateID = &templateID.Int64
	}
	if recurringID.Valid {
		t.RecurringRuleID = &recurringID.Int64
	}
	if subcolumnID.Valid {
		t.SubColumnID = &subcolumnID.Int64
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &t.Checklist)
	}
	if sizeEstimate.Valid {
		val := int(sizeEstimate.Int64)
		t.SizeEstimate = &val
	}
	if createdBy.Valid {
		t.CreatedBy = &createdBy.Int64
	}
	if due.Valid {
		t.DueDate = &due.Time
	}
	if closed.Valid {
		t.ClosedAt = &closed.Time
	}
	t.IsArchived = archived == 1
	return t, nil
}

func (s *SQLiteStore) getTaskTx(ctx context.Context, tx *sql.Tx, taskID int64) (*tasks.Task, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, board_id, column_id, subcolumn_id, title, description, result, external_link, business_customer, size_estimate, status, priority, template_id, recurring_rule_id, checklist, created_by, due_date, created_at, updated_at, closed_at, is_archived, position
		FROM tasks WHERE id=?`, taskID)
	var t tasks.Task
	var createdBy sql.NullInt64
	var templateID sql.NullInt64
	var recurringID sql.NullInt64
	var checklistRaw sql.NullString
	var due sql.NullTime
	var closed sql.NullTime
	var archived int
	var sizeEstimate sql.NullInt64
	var subcolumnID sql.NullInt64
	if err := row.Scan(&t.ID, &t.BoardID, &t.ColumnID, &subcolumnID, &t.Title, &t.Description, &t.Result, &t.ExternalLink, &t.BusinessCustomer, &sizeEstimate, &t.Status, &t.Priority, &templateID, &recurringID, &checklistRaw, &createdBy, &due, &t.CreatedAt, &t.UpdatedAt, &closed, &archived, &t.Position); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if templateID.Valid {
		t.TemplateID = &templateID.Int64
	}
	if recurringID.Valid {
		t.RecurringRuleID = &recurringID.Int64
	}
	if subcolumnID.Valid {
		t.SubColumnID = &subcolumnID.Int64
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &t.Checklist)
	}
	if sizeEstimate.Valid {
		val := int(sizeEstimate.Int64)
		t.SizeEstimate = &val
	}
	if createdBy.Valid {
		t.CreatedBy = &createdBy.Int64
	}
	if due.Valid {
		t.DueDate = &due.Time
	}
	if closed.Valid {
		t.ClosedAt = &closed.Time
	}
	t.IsArchived = archived == 1
	return &t, nil
}

func (s *SQLiteStore) archiveTaskTx(ctx context.Context, tx *sql.Tx, taskID int64, userID int64) (*tasks.Task, error) {
	task, err := s.getTaskTx(ctx, tx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, tasks.ErrNotFound
	}
	if task.IsArchived {
		return nil, tasks.ErrConflict
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO task_archive_entries(task_id, archived_at, archived_by, archived_board_id, archived_column_id, archived_subcolumn_id, original_position, restored_at, restored_by)
		VALUES(?,?,?,?,?,?,?,NULL,NULL)
		ON CONFLICT(task_id) DO UPDATE SET
			archived_at=excluded.archived_at,
			archived_by=excluded.archived_by,
			archived_board_id=excluded.archived_board_id,
			archived_column_id=excluded.archived_column_id,
			archived_subcolumn_id=excluded.archived_subcolumn_id,
			original_position=excluded.original_position,
			restored_at=NULL,
			restored_by=NULL
	`, task.ID, now, userID, task.BoardID, task.ColumnID, nullableID(task.SubColumnID), task.Position); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tasks SET is_archived=1, updated_at=? WHERE id=?`, now, taskID); err != nil {
		return nil, err
	}
	task.IsArchived = true
	task.UpdatedAt = now
	return task, nil
}

func scanArchivedTaskRow(rows *sql.Rows) (tasks.TaskArchiveEntry, error) {
	var entry tasks.TaskArchiveEntry
	var createdBy sql.NullInt64
	var templateID sql.NullInt64
	var recurringID sql.NullInt64
	var checklistRaw sql.NullString
	var due sql.NullTime
	var closed sql.NullTime
	var archived int
	var sizeEstimate sql.NullInt64
	var subcolumnID sql.NullInt64
	var archivedBy sql.NullInt64
	var archivedSubColumnID sql.NullInt64
	var restoredAt sql.NullTime
	if err := rows.Scan(
		&entry.ID, &entry.BoardID, &entry.ColumnID, &subcolumnID, &entry.Title, &entry.Description, &entry.Result, &entry.ExternalLink, &entry.BusinessCustomer, &sizeEstimate,
		&entry.Status, &entry.Priority, &templateID, &recurringID, &checklistRaw, &createdBy, &due, &entry.CreatedAt, &entry.UpdatedAt, &closed,
		&archived, &entry.Position,
		&entry.ArchivedAt, &archivedBy, &entry.ArchivedBoardID, &entry.ArchivedColumnID, &archivedSubColumnID, &entry.OriginalPosition, &restoredAt,
	); err != nil {
		return entry, err
	}
	if templateID.Valid {
		entry.TemplateID = &templateID.Int64
	}
	if recurringID.Valid {
		entry.RecurringRuleID = &recurringID.Int64
	}
	if subcolumnID.Valid {
		entry.SubColumnID = &subcolumnID.Int64
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &entry.Checklist)
	}
	if sizeEstimate.Valid {
		value := int(sizeEstimate.Int64)
		entry.SizeEstimate = &value
	}
	if createdBy.Valid {
		entry.CreatedBy = &createdBy.Int64
	}
	if due.Valid {
		entry.DueDate = &due.Time
	}
	if closed.Valid {
		entry.ClosedAt = &closed.Time
	}
	entry.IsArchived = archived == 1
	if archivedBy.Valid {
		entry.ArchivedBy = &archivedBy.Int64
	}
	if archivedSubColumnID.Valid {
		entry.ArchivedSubColumnID = &archivedSubColumnID.Int64
	}
	if restoredAt.Valid {
		entry.RestoredAt = &restoredAt.Time
	}
	return entry, nil
}
