-- +goose Up

-- Ensure Postgres auto-generates `findings.id` for deployments that created the table
-- without SERIAL/IDENTITY (e.g. `id INTEGER PRIMARY KEY`).
--
-- Note: avoid `DO $$ ... $$` here because goose splits SQL by semicolon.
CREATE SEQUENCE IF NOT EXISTS findings_id_seq;
ALTER SEQUENCE findings_id_seq OWNED BY findings.id;
ALTER TABLE findings ALTER COLUMN id SET DEFAULT nextval('findings_id_seq');
WITH m AS (SELECT MAX(id) AS max_id FROM findings)
SELECT setval('findings_id_seq', GREATEST(COALESCE(max_id, 1), 1), max_id IS NOT NULL)
FROM m;

-- +goose Down

ALTER TABLE IF EXISTS findings ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS findings_id_seq;
