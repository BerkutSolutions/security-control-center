-- +goose Up

ALTER TABLE backups_artifacts ADD COLUMN IF NOT EXISTS checksum TEXT;
ALTER TABLE backups_runs ADD COLUMN IF NOT EXISTS checksum TEXT;

CREATE INDEX IF NOT EXISTS idx_backups_artifacts_checksum ON backups_artifacts(checksum);

-- +goose Down
SELECT 1;
