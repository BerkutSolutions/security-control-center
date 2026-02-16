package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *docsStore) CreateDocExportApproval(ctx context.Context, item *DocExportApproval) (int64, error) {
	if item == nil || item.DocID == 0 || item.RequestedBy == 0 || item.ApprovedBy == 0 {
		return 0, errors.New("invalid export approval")
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.ExpiresAt.IsZero() {
		item.ExpiresAt = now.Add(30 * time.Minute)
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO doc_export_approvals(doc_id, requested_by, approved_by, reason, created_at, expires_at, consumed_at)
		VALUES(?,?,?,?,?,?,NULL)`,
		item.DocID, item.RequestedBy, item.ApprovedBy, strings.TrimSpace(item.Reason), item.CreatedAt, item.ExpiresAt)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	item.ID = id
	return id, nil
}

func (s *docsStore) ConsumeDocExportApproval(ctx context.Context, docID, requestedBy int64) (*DocExportApproval, error) {
	if docID == 0 || requestedBy == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	row := s.db.QueryRowContext(ctx, `
		SELECT id, doc_id, requested_by, approved_by, reason, created_at, expires_at, consumed_at
		FROM doc_export_approvals
		WHERE doc_id=? AND requested_by=? AND consumed_at IS NULL AND expires_at>?
		ORDER BY created_at DESC
		LIMIT 1`, docID, requestedBy, now)
	var item DocExportApproval
	var consumed sql.NullTime
	if err := row.Scan(&item.ID, &item.DocID, &item.RequestedBy, &item.ApprovedBy, &item.Reason, &item.CreatedAt, &item.ExpiresAt, &consumed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if item.ApprovedBy == requestedBy {
		return nil, nil
	}
	if consumed.Valid {
		item.ConsumedAt = &consumed.Time
		return &item, nil
	}
	usedAt := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE doc_export_approvals SET consumed_at=? WHERE id=? AND consumed_at IS NULL`, usedAt, item.ID); err != nil {
		return nil, err
	}
	item.ConsumedAt = &usedAt
	return &item, nil
}
