-- +goose Up

CREATE TABLE IF NOT EXISTS app_module_state (
	module_id TEXT PRIMARY KEY,
	applied_schema_version INTEGER NOT NULL DEFAULT 0,
	applied_behavior_version INTEGER NOT NULL DEFAULT 0,
	initialized_at TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_app_module_state_updated_at ON app_module_state(updated_at);

-- +goose Down

DROP INDEX IF EXISTS idx_app_module_state_updated_at;
DROP TABLE IF EXISTS app_module_state;

