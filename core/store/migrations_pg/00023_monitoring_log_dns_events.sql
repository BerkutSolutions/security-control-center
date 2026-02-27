-- +goose Up
ALTER TABLE monitoring_settings
  ADD COLUMN IF NOT EXISTS log_dns_events INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE monitoring_settings
  DROP COLUMN IF EXISTS log_dns_events;

