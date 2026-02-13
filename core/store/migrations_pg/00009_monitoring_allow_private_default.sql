-- +goose Up
ALTER TABLE monitoring_settings ALTER COLUMN allow_private_networks SET DEFAULT 1;
UPDATE monitoring_settings
SET allow_private_networks = 1,
    updated_at = NOW()
WHERE allow_private_networks = 0;

-- +goose Down
UPDATE monitoring_settings
SET allow_private_networks = 0,
    updated_at = NOW();
ALTER TABLE monitoring_settings ALTER COLUMN allow_private_networks SET DEFAULT 0;