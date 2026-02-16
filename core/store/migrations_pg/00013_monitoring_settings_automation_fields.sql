-- +goose Up
ALTER TABLE monitoring_settings ADD COLUMN IF NOT EXISTS auto_task_on_down INTEGER NOT NULL DEFAULT 1;
ALTER TABLE monitoring_settings ADD COLUMN IF NOT EXISTS auto_tls_incident INTEGER NOT NULL DEFAULT 1;
ALTER TABLE monitoring_settings ADD COLUMN IF NOT EXISTS auto_tls_incident_days INTEGER NOT NULL DEFAULT 14;

-- +goose Down
ALTER TABLE monitoring_settings DROP COLUMN IF EXISTS auto_tls_incident_days;
ALTER TABLE monitoring_settings DROP COLUMN IF EXISTS auto_tls_incident;
ALTER TABLE monitoring_settings DROP COLUMN IF EXISTS auto_task_on_down;
