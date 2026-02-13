-- +goose Up

CREATE TABLE IF NOT EXISTS backup_plans (
		id BIGINT PRIMARY KEY,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		cron_expression TEXT NOT NULL DEFAULT '0 2 * * *',
		retention_days INTEGER NOT NULL DEFAULT 30,
		keep_last_successful INTEGER NOT NULL DEFAULT 5,
		include_files BOOLEAN NOT NULL DEFAULT FALSE,
		last_auto_run_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

INSERT INTO backup_plans(id, enabled, cron_expression, retention_days, keep_last_successful, include_files, created_at, updated_at)
VALUES(1, FALSE, '0 2 * * *', 30, 5, FALSE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS backup_plans;

