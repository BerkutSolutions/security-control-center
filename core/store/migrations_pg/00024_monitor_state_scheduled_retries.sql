-- +goose Up
ALTER TABLE monitor_state ADD COLUMN IF NOT EXISTS retry_at TIMESTAMP;
ALTER TABLE monitor_state ADD COLUMN IF NOT EXISTS retry_attempt INTEGER NOT NULL DEFAULT 0;
ALTER TABLE monitor_state ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMP;
ALTER TABLE monitor_state ADD COLUMN IF NOT EXISTS last_error_kind TEXT NOT NULL DEFAULT '';

-- +goose Down
-- (no-op) columns are kept for backward compatibility
