package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) CreateTaskTemplate(ctx context.Context, tpl *tasks.TaskTemplate) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_templates(board_id, column_id, title_template, description_template, priority, default_assignees, default_due_days, checklist_template, links_template, is_active, created_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		tpl.BoardID, tpl.ColumnID, tpl.TitleTemplate, tpl.DescriptionTemplate, tpl.Priority,
		marshalJSON(tpl.DefaultAssignees), tpl.DefaultDueDays, marshalJSON(tpl.ChecklistTemplate), marshalJSON(tpl.LinksTemplate),
		boolToInt(tpl.IsActive), nullableID(tpl.CreatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	tpl.ID = id
	tpl.CreatedAt = now
	tpl.UpdatedAt = now
	return id, nil
}

func (s *SQLStore) UpdateTaskTemplate(ctx context.Context, tpl *tasks.TaskTemplate) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_templates SET board_id=?, column_id=?, title_template=?, description_template=?, priority=?, default_assignees=?, default_due_days=?, checklist_template=?, links_template=?, is_active=?, updated_at=? WHERE id=?`,
		tpl.BoardID, tpl.ColumnID, tpl.TitleTemplate, tpl.DescriptionTemplate, tpl.Priority,
		marshalJSON(tpl.DefaultAssignees), tpl.DefaultDueDays, marshalJSON(tpl.ChecklistTemplate), marshalJSON(tpl.LinksTemplate),
		boolToInt(tpl.IsActive), now, tpl.ID)
	if err == nil {
		tpl.UpdatedAt = now
	}
	return err
}

func (s *SQLStore) DeleteTaskTemplate(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM task_templates WHERE id=?`, id)
	return err
}

func (s *SQLStore) GetTaskTemplate(ctx context.Context, id int64) (*tasks.TaskTemplate, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, board_id, column_id, title_template, description_template, priority, default_assignees, default_due_days, checklist_template, links_template, is_active, created_by, created_at, updated_at
		FROM task_templates WHERE id=?`, id)
	return scanTaskTemplate(row)
}

func (s *SQLStore) ListTaskTemplates(ctx context.Context, filter tasks.TaskTemplateFilter) ([]tasks.TaskTemplate, error) {
	clauses := []string{}
	args := []any{}
	if filter.BoardID > 0 {
		clauses = append(clauses, "board_id=?")
		args = append(args, filter.BoardID)
	}
	if !filter.IncludeInactive {
		clauses = append(clauses, "is_active=1")
	}
	query := `
		SELECT id, board_id, column_id, title_template, description_template, priority, default_assignees, default_due_days, checklist_template, links_template, is_active, created_by, created_at, updated_at
		FROM task_templates`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskTemplate
	for rows.Next() {
		tpl, err := scanTaskTemplateRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, tpl)
	}
	return res, rows.Err()
}

func scanTaskTemplate(row *sql.Row) (*tasks.TaskTemplate, error) {
	var tpl tasks.TaskTemplate
	var assigneesRaw sql.NullString
	var checklistRaw sql.NullString
	var linksRaw sql.NullString
	var createdBy sql.NullInt64
	var active int
	if err := row.Scan(&tpl.ID, &tpl.BoardID, &tpl.ColumnID, &tpl.TitleTemplate, &tpl.DescriptionTemplate, &tpl.Priority, &assigneesRaw, &tpl.DefaultDueDays, &checklistRaw, &linksRaw, &active, &createdBy, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		tpl.CreatedBy = &createdBy.Int64
	}
	if assigneesRaw.Valid {
		unmarshalJSON(assigneesRaw.String, &tpl.DefaultAssignees)
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &tpl.ChecklistTemplate)
	}
	if linksRaw.Valid {
		unmarshalJSON(linksRaw.String, &tpl.LinksTemplate)
	}
	tpl.IsActive = active == 1
	return &tpl, nil
}

func scanTaskTemplateRow(rows *sql.Rows) (tasks.TaskTemplate, error) {
	var tpl tasks.TaskTemplate
	var assigneesRaw sql.NullString
	var checklistRaw sql.NullString
	var linksRaw sql.NullString
	var createdBy sql.NullInt64
	var active int
	if err := rows.Scan(&tpl.ID, &tpl.BoardID, &tpl.ColumnID, &tpl.TitleTemplate, &tpl.DescriptionTemplate, &tpl.Priority, &assigneesRaw, &tpl.DefaultDueDays, &checklistRaw, &linksRaw, &active, &createdBy, &tpl.CreatedAt, &tpl.UpdatedAt); err != nil {
		return tpl, err
	}
	if createdBy.Valid {
		tpl.CreatedBy = &createdBy.Int64
	}
	if assigneesRaw.Valid {
		unmarshalJSON(assigneesRaw.String, &tpl.DefaultAssignees)
	}
	if checklistRaw.Valid {
		unmarshalJSON(checklistRaw.String, &tpl.ChecklistTemplate)
	}
	if linksRaw.Valid {
		unmarshalJSON(linksRaw.String, &tpl.LinksTemplate)
	}
	tpl.IsActive = active == 1
	return tpl, nil
}
