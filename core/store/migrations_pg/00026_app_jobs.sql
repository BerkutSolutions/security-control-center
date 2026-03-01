-- +goose Up

CREATE TABLE IF NOT EXISTS app_jobs (
  id BIGSERIAL PRIMARY KEY,
  type TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT 'module',
  module_id TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL DEFAULT 'partial',
  status TEXT NOT NULL DEFAULT 'queued',
  progress INTEGER NOT NULL DEFAULT 0,
  started_by TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  log_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_app_jobs_status ON app_jobs(status);
CREATE INDEX IF NOT EXISTS idx_app_jobs_created_at ON app_jobs(created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_app_jobs_created_at;
DROP INDEX IF EXISTS idx_app_jobs_status;
DROP TABLE IF EXISTS app_jobs;

