-- +goose Up
ALTER TABLE monitoring_settings
	ADD COLUMN IF NOT EXISTS tls_expiring_rules_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE monitoring_settings
	DROP COLUMN IF EXISTS tls_expiring_rules_json;
