-- +goose Up

-- Adds incident scoring fields to monitoring state and settings.

ALTER TABLE monitor_state
  ADD COLUMN IF NOT EXISTS incident_score DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS incident_score_updated_at TIMESTAMP,
  ADD COLUMN IF NOT EXISTS incident_score_reasons TEXT NOT NULL DEFAULT '[]';

ALTER TABLE monitoring_settings
  ADD COLUMN IF NOT EXISTS incident_scoring_enabled INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS incident_score_open_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.85,
  ADD COLUMN IF NOT EXISTS incident_score_close_threshold DOUBLE PRECISION NOT NULL DEFAULT 0.25,
  ADD COLUMN IF NOT EXISTS incident_score_open_confirmations INTEGER NOT NULL DEFAULT 2;

-- +goose Down

ALTER TABLE monitoring_settings
  DROP COLUMN IF EXISTS incident_score_open_confirmations,
  DROP COLUMN IF EXISTS incident_score_close_threshold,
  DROP COLUMN IF EXISTS incident_score_open_threshold,
  DROP COLUMN IF EXISTS incident_scoring_enabled;

ALTER TABLE monitor_state
  DROP COLUMN IF EXISTS incident_score_reasons,
  DROP COLUMN IF EXISTS incident_score_updated_at,
  DROP COLUMN IF EXISTS incident_score;
