-- +goose Up

ALTER TABLE app_runtime_settings
    ADD COLUMN IF NOT EXISTS behavior_model_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS user_behavior_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    event_type TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    ip TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_behavior_events_user_ts ON user_behavior_events(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_user_behavior_events_type_ts ON user_behavior_events(event_type, created_at);

CREATE TABLE IF NOT EXISTS user_behavior_risk_state (
    user_id BIGINT PRIMARY KEY,
    stepup_required BOOLEAN NOT NULL DEFAULT FALSE,
    password_verified BOOLEAN NOT NULL DEFAULT FALSE,
    failed_stepups INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    last_triggered_at TIMESTAMPTZ,
    last_verified_at TIMESTAMPTZ,
    last_risk_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down

DROP TABLE IF EXISTS user_behavior_risk_state;
DROP INDEX IF EXISTS idx_user_behavior_events_type_ts;
DROP INDEX IF EXISTS idx_user_behavior_events_user_ts;
DROP TABLE IF EXISTS user_behavior_events;
ALTER TABLE app_runtime_settings
    DROP COLUMN IF EXISTS behavior_model_enabled;
