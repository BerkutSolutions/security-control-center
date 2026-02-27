package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) createTaskTx(ctx context.Context, tx *sql.Tx, task *tasks.Task, assignments []int64) (int64, error) {
	var colBoardID int64
	var colName string
	if err := tx.QueryRowContext(ctx, `SELECT board_id, name FROM task_columns WHERE id=?`, task.ColumnID).Scan(&colBoardID, &colName); err != nil {
		return 0, err
	}
	if task.SubColumnID != nil {
		var ownerColumnID int64
		if err := tx.QueryRowContext(ctx, `SELECT column_id FROM task_subcolumns WHERE id=? AND is_active=1`, *task.SubColumnID).Scan(&ownerColumnID); err != nil {
			return 0, err
		}
		if ownerColumnID != task.ColumnID {
			return 0, fmt.Errorf("subcolumn column mismatch")
		}
	}
	if task.BoardID == 0 {
		task.BoardID = colBoardID
	}
	if task.BoardID != colBoardID {
		return 0, fmt.Errorf("column board mismatch")
	}
	if strings.TrimSpace(task.Status) == "" {
		task.Status = colName
	}
	if task.Position <= 0 {
		var row *sql.Row
		if task.SubColumnID != nil {
			row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id=?`, task.ColumnID, *task.SubColumnID)
		} else {
			row = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM tasks WHERE column_id=? AND subcolumn_id IS NULL`, task.ColumnID)
		}
		var max int
		if err := row.Scan(&max); err != nil {
			return 0, err
		}
		task.Position = max + 1
	}
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO tasks(board_id, column_id, subcolumn_id, title, description, result, external_link, business_customer, size_estimate, status, priority, template_id, recurring_rule_id, checklist, created_by, due_date, created_at, updated_at, closed_at, is_archived, position)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		task.BoardID, task.ColumnID, nullableID(task.SubColumnID), task.Title, task.Description, task.Result, task.ExternalLink, task.BusinessCustomer, nullableInt(task.SizeEstimate), task.Status, task.Priority,
		nullableID(task.TemplateID), nullableID(task.RecurringRuleID), marshalJSON(task.Checklist),
		nullableID(task.CreatedBy), nullableTime(task.DueDate), now, now, nullableTime(task.ClosedAt), boolToInt(task.IsArchived), task.Position)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now
	if len(assignments) > 0 {
		for _, uid := range assignments {
			if uid == 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task_assignments(task_id, user_id, assigned_at, assigned_by)
				VALUES(?,?,?,?)`, id, uid, now, nullableID(task.CreatedBy)); err != nil {
				return 0, err
			}
		}
	}
	return id, nil
}

func addEntityLinkTx(ctx context.Context, tx *sql.Tx, link *tasks.Link, createdAt time.Time) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		INSERT INTO entity_links(source_type, source_id, target_type, target_id, created_at)
		VALUES(?,?,?,?,?)`,
		strings.ToLower(strings.TrimSpace(link.SourceType)),
		strings.TrimSpace(link.SourceID),
		strings.ToLower(strings.TrimSpace(link.TargetType)),
		strings.TrimSpace(link.TargetID),
		createdAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	link.ID = id
	link.CreatedAt = createdAt
	return id, nil
}
