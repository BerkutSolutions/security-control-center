-- +goose Up
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS template_text TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS quiet_hours_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS quiet_hours_start TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS quiet_hours_end TEXT NOT NULL DEFAULT '';
ALTER TABLE notification_channels ADD COLUMN IF NOT EXISTS quiet_hours_tz TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE notification_channels DROP COLUMN IF EXISTS quiet_hours_tz;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS quiet_hours_end;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS quiet_hours_start;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS quiet_hours_enabled;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS template_text;
