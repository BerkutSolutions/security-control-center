-- +goose Up

-- Adds fields required for the 3-state HMM (Normal/Degraded/Outage) incident scoring model.

ALTER TABLE monitor_state
  ADD COLUMN IF NOT EXISTS incident_score_posterior TEXT NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS incident_score_state TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS incident_score_observation TEXT NOT NULL DEFAULT '';

ALTER TABLE monitoring_settings
  ADD COLUMN IF NOT EXISTS incident_scoring_model TEXT NOT NULL DEFAULT 'heuristic';

-- +goose Down

ALTER TABLE monitoring_settings
  DROP COLUMN IF EXISTS incident_scoring_model;

ALTER TABLE monitor_state
  DROP COLUMN IF EXISTS incident_score_observation,
  DROP COLUMN IF EXISTS incident_score_state,
  DROP COLUMN IF EXISTS incident_score_posterior;
