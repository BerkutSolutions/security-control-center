-- +goose Up

ALTER TABLE backup_plans
	ADD COLUMN IF NOT EXISTS schedule_type TEXT NOT NULL DEFAULT 'daily',
	ADD COLUMN IF NOT EXISTS schedule_weekday INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN IF NOT EXISTS schedule_month_anchor TEXT NOT NULL DEFAULT 'start',
	ADD COLUMN IF NOT EXISTS schedule_hour INTEGER NOT NULL DEFAULT 2,
	ADD COLUMN IF NOT EXISTS schedule_minute INTEGER NOT NULL DEFAULT 0;

UPDATE backup_plans
SET
	schedule_type = COALESCE(NULLIF(schedule_type, ''), 'daily'),
	schedule_weekday = CASE WHEN schedule_weekday BETWEEN 0 AND 6 THEN schedule_weekday ELSE 0 END,
	schedule_month_anchor = CASE WHEN schedule_month_anchor IN ('start', 'end') THEN schedule_month_anchor ELSE 'start' END,
	schedule_hour = CASE WHEN schedule_hour BETWEEN 0 AND 23 THEN schedule_hour ELSE 2 END,
	schedule_minute = CASE WHEN schedule_minute BETWEEN 0 AND 59 THEN schedule_minute ELSE 0 END;

-- +goose Down

ALTER TABLE backup_plans
	DROP COLUMN IF EXISTS schedule_minute,
	DROP COLUMN IF EXISTS schedule_hour,
	DROP COLUMN IF EXISTS schedule_month_anchor,
	DROP COLUMN IF EXISTS schedule_weekday,
	DROP COLUMN IF EXISTS schedule_type;
