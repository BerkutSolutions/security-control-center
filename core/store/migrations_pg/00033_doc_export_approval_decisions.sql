-- +goose Up

CREATE TABLE IF NOT EXISTS doc_export_approval_decisions (
  id BIGSERIAL PRIMARY KEY,
  approval_id BIGINT NOT NULL UNIQUE REFERENCES doc_export_approvals(id) ON DELETE CASCADE,
  decision TEXT NOT NULL,
  comment TEXT NOT NULL DEFAULT '',
  decided_by BIGINT NOT NULL,
  decided_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_doc_export_approval_decisions_approval
  ON doc_export_approval_decisions(approval_id);

-- +goose Down

DROP INDEX IF EXISTS idx_doc_export_approval_decisions_approval;
DROP TABLE IF EXISTS doc_export_approval_decisions;
