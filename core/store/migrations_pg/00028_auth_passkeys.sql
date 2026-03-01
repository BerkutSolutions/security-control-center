-- +goose Up

-- Auth: Passkeys (WebAuthn)

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS webauthn_enabled INTEGER NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS user_passkeys (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL DEFAULT '',
  credential_id TEXT NOT NULL UNIQUE,
  public_key BYTEA NOT NULL,
  attestation_type TEXT NOT NULL DEFAULT '',
  transports_json TEXT NOT NULL DEFAULT '[]',
  aaguid BYTEA NULL,
  sign_count BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_used_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_user_passkeys_user_id ON user_passkeys(user_id);

CREATE TABLE IF NOT EXISTS webauthn_challenges (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  user_id BIGINT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_data_json TEXT NOT NULL,
  ip TEXT NULL,
  user_agent TEXT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_expires_at ON webauthn_challenges(expires_at);
CREATE INDEX IF NOT EXISTS idx_webauthn_challenges_user_id ON webauthn_challenges(user_id);

-- +goose Down

DROP INDEX IF EXISTS idx_webauthn_challenges_user_id;
DROP INDEX IF EXISTS idx_webauthn_challenges_expires_at;
DROP TABLE IF EXISTS webauthn_challenges;

DROP INDEX IF EXISTS idx_user_passkeys_user_id;
DROP TABLE IF EXISTS user_passkeys;

ALTER TABLE users DROP COLUMN IF EXISTS webauthn_enabled;
