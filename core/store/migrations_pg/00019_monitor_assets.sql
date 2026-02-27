-- +goose Up

CREATE TABLE IF NOT EXISTS monitor_assets (
	monitor_id INTEGER NOT NULL,
	asset_id INTEGER NOT NULL,
	created_by INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY(monitor_id, asset_id),
	FOREIGN KEY(monitor_id) REFERENCES monitors(id) ON DELETE CASCADE,
	FOREIGN KEY(asset_id) REFERENCES assets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_monitor_assets_monitor ON monitor_assets(monitor_id);
CREATE INDEX IF NOT EXISTS idx_monitor_assets_asset ON monitor_assets(asset_id);

-- +goose Down

DROP INDEX IF EXISTS idx_monitor_assets_asset;
DROP INDEX IF EXISTS idx_monitor_assets_monitor;
DROP TABLE IF EXISTS monitor_assets;
