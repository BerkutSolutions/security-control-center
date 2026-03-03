-- +goose Up

-- Adds configurable confirmation count for sending "UP recovered" notifications.

ALTER TABLE monitoring_settings
  ADD COLUMN IF NOT EXISTS notify_up_confirmations INTEGER NOT NULL DEFAULT 2;

-- +goose Down

ALTER TABLE monitoring_settings
  DROP COLUMN IF EXISTS notify_up_confirmations;

