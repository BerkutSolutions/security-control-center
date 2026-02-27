-- +goose Up

CREATE TABLE IF NOT EXISTS findings (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description_md TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open',
	severity TEXT NOT NULL DEFAULT 'medium',
	finding_type TEXT NOT NULL DEFAULT 'other',
	owner TEXT NOT NULL DEFAULT '',
	due_at TIMESTAMP,
	tags_json TEXT NOT NULL DEFAULT '[]',
	created_by INTEGER,
	updated_by INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	version INTEGER NOT NULL DEFAULT 1,
	deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_findings_title ON findings(title);
CREATE INDEX IF NOT EXISTS idx_findings_status ON findings(status);
CREATE INDEX IF NOT EXISTS idx_findings_severity ON findings(severity);
CREATE INDEX IF NOT EXISTS idx_findings_type ON findings(finding_type);
CREATE INDEX IF NOT EXISTS idx_findings_deleted_at ON findings(deleted_at);
CREATE INDEX IF NOT EXISTS idx_findings_created_at ON findings(created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_findings_created_at;
DROP INDEX IF EXISTS idx_findings_deleted_at;
DROP INDEX IF EXISTS idx_findings_type;
DROP INDEX IF EXISTS idx_findings_severity;
DROP INDEX IF EXISTS idx_findings_status;
DROP INDEX IF EXISTS idx_findings_title;
DROP TABLE IF EXISTS findings;
