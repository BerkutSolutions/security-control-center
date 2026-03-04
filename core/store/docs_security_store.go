package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
		  AND EXISTS (
		    SELECT 1 FROM doc_export_approval_decisions d
		    WHERE d.approval_id=doc_export_approvals.id AND d.decision='approve'
		  )
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

func (s *docsStore) ListActiveDocExportApprovalsForUser(ctx context.Context, requestedBy int64) ([]DocExportApproval, error) {
	if requestedBy <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, doc_id, requested_by, approved_by, reason, created_at, expires_at
		FROM doc_export_approvals
		WHERE requested_by=? AND consumed_at IS NULL AND expires_at>?
		  AND EXISTS (
		    SELECT 1 FROM doc_export_approval_decisions d
		    WHERE d.approval_id=doc_export_approvals.id AND d.decision='approve'
		  )
		ORDER BY created_at DESC`, requestedBy, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("list active export approvals: %w", err)
	}
	defer rows.Close()
	items := make([]DocExportApproval, 0, 16)
	for rows.Next() {
		var item DocExportApproval
		if err := rows.Scan(&item.ID, &item.DocID, &item.RequestedBy, &item.ApprovedBy, &item.Reason, &item.CreatedAt, &item.ExpiresAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *docsStore) ListDocExportApprovalsForActor(ctx context.Context, actorID int64) ([]DocExportApproval, error) {
	if actorID <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, doc_id, requested_by, approved_by, reason, created_at, expires_at, consumed_at
		FROM doc_export_approvals
		WHERE (requested_by=? OR approved_by=?)
		ORDER BY created_at DESC`, actorID, actorID)
	if err != nil {
		return nil, fmt.Errorf("list export approvals for actor: %w", err)
	}
	defer rows.Close()
	items := make([]DocExportApproval, 0, 16)
	for rows.Next() {
		var item DocExportApproval
		var consumed sql.NullTime
		if err := rows.Scan(&item.ID, &item.DocID, &item.RequestedBy, &item.ApprovedBy, &item.Reason, &item.CreatedAt, &item.ExpiresAt, &consumed); err != nil {
			return nil, err
		}
		if consumed.Valid {
			v := consumed.Time
			item.ConsumedAt = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *docsStore) GetDocExportApproval(ctx context.Context, id int64) (*DocExportApproval, error) {
	if id <= 0 {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, doc_id, requested_by, approved_by, reason, created_at, expires_at, consumed_at
		FROM doc_export_approvals
		WHERE id=?`, id)
	var item DocExportApproval
	var consumed sql.NullTime
	if err := row.Scan(&item.ID, &item.DocID, &item.RequestedBy, &item.ApprovedBy, &item.Reason, &item.CreatedAt, &item.ExpiresAt, &consumed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if consumed.Valid {
		v := consumed.Time
		item.ConsumedAt = &v
	}
	return &item, nil
}

func (s *docsStore) GetDocExportApprovalDecision(ctx context.Context, approvalID int64) (*DocExportApprovalDecision, error) {
	if approvalID <= 0 {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, approval_id, decision, comment, decided_by, decided_at
		FROM doc_export_approval_decisions
		WHERE approval_id=?`, approvalID)
	var item DocExportApprovalDecision
	if err := row.Scan(&item.ID, &item.ApprovalID, &item.Decision, &item.Comment, &item.DecidedBy, &item.DecidedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *docsStore) SaveDocExportApprovalDecision(ctx context.Context, item *DocExportApprovalDecision) error {
	if item == nil || item.ApprovalID <= 0 || item.DecidedBy <= 0 {
		return errors.New("invalid export approval decision")
	}
	decision := strings.ToLower(strings.TrimSpace(item.Decision))
	if decision != "approve" && decision != "reject" {
		return errors.New("invalid decision")
	}
	if item.DecidedAt.IsZero() {
		item.DecidedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE doc_export_approval_decisions
		SET decision=?, comment=?, decided_by=?, decided_at=?
		WHERE approval_id=?`,
		decision, strings.TrimSpace(item.Comment), item.DecidedBy, item.DecidedAt, item.ApprovalID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff > 0 {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO doc_export_approval_decisions(approval_id, decision, comment, decided_by, decided_at)
		VALUES(?,?,?,?,?)`,
		item.ApprovalID, decision, strings.TrimSpace(item.Comment), item.DecidedBy, item.DecidedAt)
	return err
}
