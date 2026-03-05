-- +goose Up

ALTER TABLE monitor_metrics ADD COLUMN IF NOT EXISTS final_url TEXT NOT NULL DEFAULT '';
ALTER TABLE monitor_metrics ADD COLUMN IF NOT EXISTS remote_ip TEXT NOT NULL DEFAULT '';
ALTER TABLE monitor_metrics ADD COLUMN IF NOT EXISTS response_headers_json TEXT NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE monitor_metrics DROP COLUMN IF EXISTS response_headers_json;
ALTER TABLE monitor_metrics DROP COLUMN IF EXISTS remote_ip;
ALTER TABLE monitor_metrics DROP COLUMN IF EXISTS final_url;
