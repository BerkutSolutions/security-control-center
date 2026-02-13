-- +goose Up

ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS description_md TEXT NOT NULL DEFAULT '';
ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS monitor_ids_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS strategy TEXT NOT NULL DEFAULT 'single';
ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS strategy_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS stopped_at TIMESTAMP;
ALTER TABLE monitor_maintenance
	ADD COLUMN IF NOT EXISTS stopped_by INTEGER;

CREATE INDEX IF NOT EXISTS idx_monitor_maintenance_active ON monitor_maintenance(is_active, strategy);

-- +goose Down

DROP INDEX IF EXISTS idx_monitor_maintenance_active;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS stopped_by;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS stopped_at;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS strategy_json;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS strategy;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS monitor_ids_json;
ALTER TABLE monitor_maintenance DROP COLUMN IF EXISTS description_md;
