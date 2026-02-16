-- +goose Up

CREATE TABLE IF NOT EXISTS app_maintenance_state (
		id BIGINT PRIMARY KEY,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		reason TEXT NOT NULL DEFAULT '',
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

INSERT INTO app_maintenance_state(id, enabled, reason, updated_at)
VALUES(1, FALSE, '', CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS app_maintenance_state;

