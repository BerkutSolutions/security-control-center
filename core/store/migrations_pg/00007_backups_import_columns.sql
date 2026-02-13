-- +goose Up

ALTER TABLE backups_artifacts ADD COLUMN IF NOT EXISTS run_id BIGINT NULL;
ALTER TABLE backups_artifacts ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local';
ALTER TABLE backups_artifacts ADD COLUMN IF NOT EXISTS created_by_user_id BIGINT NULL;
ALTER TABLE backups_artifacts ADD COLUMN IF NOT EXISTS origin_filename TEXT NULL;

-- +goose StatementBegin
DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_backups_artifacts_run'
	) THEN
		ALTER TABLE backups_artifacts
			ADD CONSTRAINT fk_backups_artifacts_run
			FOREIGN KEY (run_id) REFERENCES backups_runs(id) ON DELETE SET NULL;
	END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_backups_artifacts_source ON backups_artifacts(source);
CREATE INDEX IF NOT EXISTS idx_backups_artifacts_run_id ON backups_artifacts(run_id);

-- +goose Down
SELECT 1;
