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

func (s *SQLiteStore) CreateTaskRecurringRule(ctx context.Context, rule *tasks.TaskRecurringRule) (int64, error) {
	now := time.Now().UTC()
	config := normalizeJSON(string(rule.ScheduleConfig), "{}")
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_recurring_rules(template_id, schedule_type, schedule_config, time_of_day, next_run_at, last_run_at, is_active, created_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		rule.TemplateID, strings.TrimSpace(rule.ScheduleType), config, strings.TrimSpace(rule.TimeOfDay),
		nullableTime(rule.NextRunAt), nullableTime(rule.LastRunAt), boolToInt(rule.IsActive), nullableID(rule.CreatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	rule.ID = id
	rule.CreatedAt = now
	rule.UpdatedAt = now
	rule.ScheduleConfig = []byte(config)
	return id, nil
}

func (s *SQLiteStore) UpdateTaskRecurringRule(ctx context.Context, rule *tasks.TaskRecurringRule) error {
	now := time.Now().UTC()
	config := normalizeJSON(string(rule.ScheduleConfig), "{}")
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_recurring_rules SET schedule_type=?, schedule_config=?, time_of_day=?, next_run_at=?, last_run_at=?, is_active=?, updated_at=? WHERE id=?`,
		strings.TrimSpace(rule.ScheduleType), config, strings.TrimSpace(rule.TimeOfDay),
		nullableTime(rule.NextRunAt), nullableTime(rule.LastRunAt), boolToInt(rule.IsActive), now, rule.ID)
	if err == nil {
		rule.UpdatedAt = now
		rule.ScheduleConfig = []byte(config)
	}
	return err
}

func (s *SQLiteStore) GetTaskRecurringRule(ctx context.Context, id int64) (*tasks.TaskRecurringRule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, template_id, schedule_type, schedule_config, time_of_day, next_run_at, last_run_at, is_active, created_by, created_at, updated_at
		FROM task_recurring_rules WHERE id=?`, id)
	return scanTaskRecurringRule(row)
}

func (s *SQLiteStore) ListTaskRecurringRules(ctx context.Context, filter tasks.TaskRecurringRuleFilter) ([]tasks.TaskRecurringRule, error) {
	clauses := []string{}
	args := []any{}
	if filter.TemplateID > 0 {
		clauses = append(clauses, "template_id=?")
		args = append(args, filter.TemplateID)
	}
	if !filter.IncludeInactive {
		clauses = append(clauses, "is_active=1")
	}
	query := `
		SELECT id, template_id, schedule_type, schedule_config, time_of_day, next_run_at, last_run_at, is_active, created_by, created_at, updated_at
		FROM task_recurring_rules`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskRecurringRule
	for rows.Next() {
		rule, err := scanTaskRecurringRuleRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, rule)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) ListDueRecurringRules(ctx context.Context, now time.Time, limit int) ([]tasks.TaskRecurringRule, error) {
	query := `
		SELECT id, template_id, schedule_type, schedule_config, time_of_day, next_run_at, last_run_at, is_active, created_by, created_at, updated_at
		FROM task_recurring_rules
		WHERE is_active=1 AND next_run_at IS NOT NULL AND next_run_at<=?
		ORDER BY next_run_at ASC`
	if limit > 0 {
		query += " LIMIT " + fmt.Sprintf("%d", limit)
	}
	rows, err := s.db.QueryContext(ctx, query, now.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskRecurringRule
	for rows.Next() {
		rule, err := scanTaskRecurringRuleRow(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, rule)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) UpdateRecurringRuleRun(ctx context.Context, ruleID int64, lastRunAt, nextRunAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_recurring_rules SET last_run_at=?, next_run_at=?, updated_at=? WHERE id=?`,
		lastRunAt.UTC(), nextRunAt.UTC(), time.Now().UTC(), ruleID)
	return err
}

func (s *SQLiteStore) CreateRecurringInstanceTask(ctx context.Context, rule *tasks.TaskRecurringRule, template *tasks.TaskTemplate, scheduledFor time.Time) (*tasks.Task, bool, error) {
	var createdTask *tasks.Task
	created := false
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var existingID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM task_recurring_instances WHERE rule_id=? AND scheduled_for=?`, rule.ID, scheduledFor.UTC()).Scan(&existingID)
		if err == nil {
			return nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		now := time.Now().UTC()
		res, err := tx.ExecContext(ctx, `
			INSERT INTO task_recurring_instances(rule_id, template_id, task_id, scheduled_for, created_at)
			VALUES(?,?,?,?,?)`,
			rule.ID, template.ID, nil, scheduledFor.UTC(), now)
		if err != nil {
			return err
		}
		instanceID, _ := res.LastInsertId()
		task := &tasks.Task{
			BoardID:        template.BoardID,
			ColumnID:       template.ColumnID,
			Title:          strings.TrimSpace(template.TitleTemplate),
			Description:    strings.TrimSpace(template.DescriptionTemplate),
			Priority:       strings.ToLower(strings.TrimSpace(template.Priority)),
			TemplateID:     &template.ID,
			RecurringRuleID: &rule.ID,
			CreatedBy:      rule.CreatedBy,
			Checklist:      append([]tasks.TaskChecklistItem{}, template.ChecklistTemplate...),
			IsArchived:     false,
		}
		if task.CreatedBy == nil {
			task.CreatedBy = template.CreatedBy
		}
		if template.DefaultDueDays > 0 {
			due := now.AddDate(0, 0, template.DefaultDueDays)
			task.DueDate = &due
		}
		links := buildTemplateLinks(template, "")
		if _, err := s.createTaskTx(ctx, tx, task, template.DefaultAssignees); err != nil {
			return err
		}
		if len(links) > 0 {
			for i := range links {
				links[i].SourceID = fmt.Sprintf("%d", task.ID)
				if _, err := addEntityLinkTx(ctx, tx, &links[i], now); err != nil {
					return err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE task_recurring_instances SET task_id=? WHERE id=?`, task.ID, instanceID); err != nil {
			return err
		}
		createdTask = task
		created = true
		return nil
	})
	if err != nil {
		if isUniqueConstraint(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return createdTask, created, nil
}

func buildTemplateLinks(template *tasks.TaskTemplate, sourceID string) []tasks.Link {
	if template == nil || len(template.LinksTemplate) == 0 {
		return nil
	}
	out := make([]tasks.Link, 0, len(template.LinksTemplate))
	for _, lt := range template.LinksTemplate {
		targetType := strings.ToLower(strings.TrimSpace(lt.TargetType))
		targetID := strings.TrimSpace(lt.TargetID)
		if targetType == "" || targetID == "" {
			continue
		}
		out = append(out, tasks.Link{
			SourceType: "task",
			SourceID:   sourceID,
			TargetType: targetType,
			TargetID:   targetID,
		})
	}
	return out
}

func scanTaskRecurringRule(row *sql.Row) (*tasks.TaskRecurringRule, error) {
	var rule tasks.TaskRecurringRule
	var configRaw sql.NullString
	var next sql.NullTime
	var last sql.NullTime
	var createdBy sql.NullInt64
	var active int
	if err := row.Scan(&rule.ID, &rule.TemplateID, &rule.ScheduleType, &configRaw, &rule.TimeOfDay, &next, &last, &active, &createdBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if configRaw.Valid {
		rule.ScheduleConfig = []byte(configRaw.String)
	}
	if next.Valid {
		rule.NextRunAt = &next.Time
	}
	if last.Valid {
		rule.LastRunAt = &last.Time
	}
	if createdBy.Valid {
		rule.CreatedBy = &createdBy.Int64
	}
	rule.IsActive = active == 1
	return &rule, nil
}

func scanTaskRecurringRuleRow(rows *sql.Rows) (tasks.TaskRecurringRule, error) {
	var rule tasks.TaskRecurringRule
	var configRaw sql.NullString
	var next sql.NullTime
	var last sql.NullTime
	var createdBy sql.NullInt64
	var active int
	if err := rows.Scan(&rule.ID, &rule.TemplateID, &rule.ScheduleType, &configRaw, &rule.TimeOfDay, &next, &last, &active, &createdBy, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return rule, err
	}
	if configRaw.Valid {
		rule.ScheduleConfig = []byte(configRaw.String)
	}
	if next.Valid {
		rule.NextRunAt = &next.Time
	}
	if last.Valid {
		rule.LastRunAt = &last.Time
	}
	if createdBy.Valid {
		rule.CreatedBy = &createdBy.Int64
	}
	rule.IsActive = active == 1
	return rule, nil
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "unique constraint")
}
