package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"berkut-scc/core/backups"
)

func (r *Repository) GetBackupPlan(ctx context.Context) (*backups.BackupPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, enabled, cron_expression, schedule_type, schedule_weekday, schedule_month_anchor, schedule_hour, schedule_minute, retention_days, keep_last_successful, include_files, created_at, updated_at, last_auto_run_at
		FROM backup_plans
		WHERE id=1
	`)
	item := backups.BackupPlan{}
	var lastAuto sql.NullTime
	err := row.Scan(
		&item.ID,
		&item.Enabled,
		&item.CronExpression,
		&item.ScheduleType,
		&item.ScheduleWeekday,
		&item.ScheduleMonthAnchor,
		&item.ScheduleHour,
		&item.ScheduleMinute,
		&item.RetentionDays,
		&item.KeepLastSuccessful,
		&item.IncludeFiles,
		&item.CreatedAt,
		&item.UpdatedAt,
		&lastAuto,
	)
	if err == sql.ErrNoRows {
		now := time.Now().UTC()
		return &backups.BackupPlan{
			ID:                  1,
			Enabled:             false,
			CronExpression:      "0 2 * * *",
			ScheduleType:        "daily",
			ScheduleWeekday:     0,
			ScheduleMonthAnchor: "start",
			ScheduleHour:        2,
			ScheduleMinute:      0,
			RetentionDays:       30,
			KeepLastSuccessful:  5,
			IncludeFiles:        false,
			CreatedAt:           now,
			UpdatedAt:           now,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	if lastAuto.Valid {
		item.LastAutoRunAt = &lastAuto.Time
	}
	return &item, nil
}

func (r *Repository) UpsertBackupPlan(ctx context.Context, plan *backups.BackupPlan) (*backups.BackupPlan, error) {
	if plan == nil {
		return nil, sql.ErrNoRows
	}
	now := time.Now().UTC()
	cronExpr := strings.TrimSpace(plan.CronExpression)
	if cronExpr == "" {
		cronExpr = "0 2 * * *"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO backup_plans(id, enabled, cron_expression, schedule_type, schedule_weekday, schedule_month_anchor, schedule_hour, schedule_minute, retention_days, keep_last_successful, include_files, created_at, updated_at)
		VALUES(1,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT (id) DO UPDATE
		SET enabled=EXCLUDED.enabled,
			cron_expression=EXCLUDED.cron_expression,
			schedule_type=EXCLUDED.schedule_type,
			schedule_weekday=EXCLUDED.schedule_weekday,
			schedule_month_anchor=EXCLUDED.schedule_month_anchor,
			schedule_hour=EXCLUDED.schedule_hour,
			schedule_minute=EXCLUDED.schedule_minute,
			retention_days=EXCLUDED.retention_days,
			keep_last_successful=EXCLUDED.keep_last_successful,
			include_files=EXCLUDED.include_files,
			updated_at=EXCLUDED.updated_at
	`, plan.Enabled, cronExpr, plan.ScheduleType, plan.ScheduleWeekday, plan.ScheduleMonthAnchor, plan.ScheduleHour, plan.ScheduleMinute, plan.RetentionDays, plan.KeepLastSuccessful, plan.IncludeFiles, now, now)
	if err != nil {
		return nil, err
	}
	return r.GetBackupPlan(ctx)
}

func (r *Repository) SetBackupPlanEnabled(ctx context.Context, enabled bool) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO backup_plans(id, enabled, cron_expression, schedule_type, schedule_weekday, schedule_month_anchor, schedule_hour, schedule_minute, retention_days, keep_last_successful, include_files, created_at, updated_at)
		VALUES(1,?, '0 2 * * *', 'daily', 0, 'start', 2, 0, 30, 5, FALSE, ?, ?)
		ON CONFLICT (id) DO UPDATE
		SET enabled=EXCLUDED.enabled, updated_at=EXCLUDED.updated_at
	`, enabled, now, now)
	return err
}

func (r *Repository) UpdateBackupPlanLastAutoRunAt(ctx context.Context, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE backup_plans SET last_auto_run_at=?, updated_at=? WHERE id=1`, at.UTC(), time.Now().UTC())
	return err
}
