-- +goose Up

CREATE UNIQUE INDEX IF NOT EXISTS idx_backups_runs_single_active
ON backups_runs ((1))
WHERE status IN ('queued', 'running');

-- +goose Down
DROP INDEX IF EXISTS idx_backups_runs_single_active;

