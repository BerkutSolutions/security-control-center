-- +goose Up

-- Adds configurable threshold for escalating transient network issues (ISSUE) into a confirmed DOWN.

ALTER TABLE monitoring_settings
  ADD COLUMN IF NOT EXISTS issue_escalate_minutes INTEGER NOT NULL DEFAULT 10;

-- +goose Down

ALTER TABLE monitoring_settings
  DROP COLUMN IF EXISTS issue_escalate_minutes;

