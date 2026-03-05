package store

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type AuditStore interface {
	Log(ctx context.Context, username, action, details string) error
	List(ctx context.Context) ([]AuditRecord, error)
	ListFiltered(ctx context.Context, since time.Time, limit int) ([]AuditRecord, error)
	ListIntegrityFiltered(ctx context.Context, since time.Time, to *time.Time, limit int) ([]AuditIntegrityRecord, error)
	CreatePurgeRequest(ctx context.Context, initiatedBy string, retentionDays int, reason string) (*AuditPurgeRequest, error)
	ListPurgeRequests(ctx context.Context, limit int) ([]AuditPurgeRequest, error)
	ApproveAndExecutePurge(ctx context.Context, requestID int64, approvedBy string) (*AuditPurgeResult, error)
}

type AuditRecord struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditIntegrityRecord struct {
	AuditRecord
	PrevHash  string `json:"prev_hash"`
	EventHash string `json:"event_hash"`
	EventSig  string `json:"event_sig"`
}

type AuditPurgeRequest struct {
	ID               int64      `json:"id"`
	InitiatedBy      string     `json:"initiated_by"`
	ApprovedBy       string     `json:"approved_by"`
	Status           string     `json:"status"`
	RetentionDays    int        `json:"retention_days"`
	CutoffAt         time.Time  `json:"cutoff_at"`
	Reason           string     `json:"reason"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        time.Time  `json:"expires_at"`
	ApprovedAt       *time.Time `json:"approved_at,omitempty"`
	ExecutedAt       *time.Time `json:"executed_at,omitempty"`
	ExecutionDetails string     `json:"execution_details"`
}

type AuditPurgeResult struct {
	RequestID int64 `json:"request_id"`
	Deleted   int64 `json:"deleted"`
}

type auditStore struct {
	db         *sql.DB
	signingKey []byte
}

func NewAuditStore(db *sql.DB) AuditStore {
	key := strings.TrimSpace(os.Getenv("BERKUT_AUDIT_SIGNING_KEY"))
	return &auditStore{db: db, signingKey: []byte(key)}
}

func (s *auditStore) Log(ctx context.Context, username, action, details string) error {
	now := time.Now().UTC()
	prevHash := ""
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(event_hash, '') FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&prevHash); err != nil && err != sql.ErrNoRows {
		return err
	}
	eventHash := hashAuditEvent(prevHash, username, action, details, now)
	eventSig := ""
	if len(s.signingKey) > 0 {
		eventSig = signAuditEventHash(eventHash, s.signingKey)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_log(username, action, details, created_at, prev_hash, event_hash, event_sig) VALUES(?,?,?,?,?,?,?)`, username, action, details, now, prevHash, eventHash, eventSig)
	if err == nil {
		return nil
	}
	if !isMissingAuditColumnErr(err) {
		return err
	}
	_, legacyErr := s.db.ExecContext(ctx, `INSERT INTO audit_log(username, action, details, created_at) VALUES(?,?,?,?)`, username, action, details, now)
	return legacyErr
}

func (s *auditStore) List(ctx context.Context) ([]AuditRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, action, details, created_at FROM audit_log ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []AuditRecord
	for rows.Next() {
		var r AuditRecord
		if err := rows.Scan(&r.ID, &r.Username, &r.Action, &r.Details, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

func (s *auditStore) ListIntegrityFiltered(ctx context.Context, since time.Time, to *time.Time, limit int) ([]AuditIntegrityRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, action, details, created_at, COALESCE(prev_hash, ''), COALESCE(event_hash, ''), COALESCE(event_sig, '')
		FROM audit_log
		WHERE created_at>=? AND (? IS NULL OR created_at<=?)
		ORDER BY created_at ASC, id ASC
		LIMIT ?`, since, nullableUTC(to), nullableUTC(to), limit)
	if err != nil {
		if !isMissingAuditColumnErr(err) {
			return nil, err
		}
		raw, legacyErr := s.ListFiltered(ctx, since, limit)
		if legacyErr != nil {
			return nil, legacyErr
		}
		out := make([]AuditIntegrityRecord, 0, len(raw))
		prev := ""
		for i := len(raw) - 1; i >= 0; i-- {
			item := raw[i]
			if to != nil && item.CreatedAt.After(*to) {
				continue
			}
			hash := hashAuditEvent(prev, item.Username, item.Action, item.Details, item.CreatedAt)
			out = append(out, AuditIntegrityRecord{
				AuditRecord: item,
				PrevHash:    prev,
				EventHash:   hash,
				EventSig:    "",
			})
			prev = hash
		}
		return out, nil
	}
	defer rows.Close()
	var res []AuditIntegrityRecord
	for rows.Next() {
		var r AuditIntegrityRecord
		if err := rows.Scan(&r.ID, &r.Username, &r.Action, &r.Details, &r.CreatedAt, &r.PrevHash, &r.EventHash, &r.EventSig); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

func nullableUTC(v *time.Time) any {
	if v == nil {
		return nil
	}
	return v.UTC()
}

func hashAuditEvent(prevHash, username, action, details string, createdAt time.Time) string {
	payload := strings.Join([]string{
		strings.TrimSpace(prevHash),
		createdAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(username),
		strings.TrimSpace(action),
		strings.TrimSpace(details),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func signAuditEventHash(eventHash string, key []byte) string {
	if strings.TrimSpace(eventHash) == "" || len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(eventHash))
	return hex.EncodeToString(mac.Sum(nil))
}

func isMissingAuditColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "prev_hash") || strings.Contains(msg, "event_hash") || strings.Contains(msg, "event_sig")
}

func (s *auditStore) CreatePurgeRequest(ctx context.Context, initiatedBy string, retentionDays int, reason string) (*AuditPurgeRequest, error) {
	if retentionDays < 1 || retentionDays > 3650 {
		return nil, errors.New("invalid retention_days")
	}
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(retentionDays) * 24 * time.Hour)
	expires := now.Add(24 * time.Hour)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_purge_requests(initiated_by, approved_by, status, retention_days, cutoff_at, reason, created_at, expires_at, execution_details)
		VALUES(?, '', 'pending', ?, ?, ?, ?, ?, '')`,
		strings.TrimSpace(initiatedBy), retentionDays, cutoff, strings.TrimSpace(reason), now, expires)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	_ = s.Log(ctx, initiatedBy, "logs.purge.requested", "request_id="+int64ToString(id)+" retention_days="+strconv.Itoa(retentionDays))
	return &AuditPurgeRequest{
		ID:            id,
		InitiatedBy:   strings.TrimSpace(initiatedBy),
		Status:        "pending",
		RetentionDays: retentionDays,
		CutoffAt:      cutoff,
		Reason:        strings.TrimSpace(reason),
		CreatedAt:     now,
		ExpiresAt:     expires,
	}, nil
}

func (s *auditStore) ListPurgeRequests(ctx context.Context, limit int) ([]AuditPurgeRequest, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, initiated_by, approved_by, status, retention_days, cutoff_at, reason, created_at, expires_at, approved_at, executed_at, execution_details
		FROM audit_purge_requests
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AuditPurgeRequest, 0)
	for rows.Next() {
		var item AuditPurgeRequest
		var approvedAt, executedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.InitiatedBy, &item.ApprovedBy, &item.Status, &item.RetentionDays, &item.CutoffAt, &item.Reason, &item.CreatedAt, &item.ExpiresAt, &approvedAt, &executedAt, &item.ExecutionDetails); err != nil {
			return nil, err
		}
		if approvedAt.Valid {
			item.ApprovedAt = &approvedAt.Time
		}
		if executedAt.Valid {
			item.ExecutedAt = &executedAt.Time
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *auditStore) ApproveAndExecutePurge(ctx context.Context, requestID int64, approvedBy string) (*AuditPurgeResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var req AuditPurgeRequest
	var approvedAt, executedAt sql.NullTime
	row := tx.QueryRowContext(ctx, `
		SELECT id, initiated_by, approved_by, status, retention_days, cutoff_at, reason, created_at, expires_at, approved_at, executed_at, execution_details
		FROM audit_purge_requests WHERE id=?`, requestID)
	if scanErr := row.Scan(&req.ID, &req.InitiatedBy, &req.ApprovedBy, &req.Status, &req.RetentionDays, &req.CutoffAt, &req.Reason, &req.CreatedAt, &req.ExpiresAt, &approvedAt, &executedAt, &req.ExecutionDetails); scanErr != nil {
		err = scanErr
		return nil, err
	}
	if strings.TrimSpace(req.Status) != "pending" {
		err = errors.New("request is not pending")
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(req.InitiatedBy), strings.TrimSpace(approvedBy)) {
		err = errors.New("second approver must differ from initiator")
		return nil, err
	}
	now := time.Now().UTC()
	if now.After(req.ExpiresAt) {
		err = errors.New("request expired")
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE audit_log_maintenance_guard SET allow_delete=1, updated_at=? WHERE id=1`, now); err != nil {
		return nil, err
	}
	delRes, delErr := tx.ExecContext(ctx, `DELETE FROM audit_log WHERE created_at < ?`, req.CutoffAt)
	if delErr != nil {
		return nil, delErr
	}
	deleted, _ := delRes.RowsAffected()
	if _, err = tx.ExecContext(ctx, `UPDATE audit_log_maintenance_guard SET allow_delete=0, updated_at=? WHERE id=1`, now); err != nil {
		return nil, err
	}
	details := "deleted=" + int64ToString(deleted)
	updRes, updErr := tx.ExecContext(ctx, `
		UPDATE audit_purge_requests
		SET status='executed', approved_by=?, approved_at=?, executed_at=?, execution_details=?
		WHERE id=? AND status='pending'`, strings.TrimSpace(approvedBy), now, now, details, requestID)
	if updErr != nil {
		return nil, updErr
	}
	affected, _ := updRes.RowsAffected()
	if affected != 1 {
		err = errors.New("request approval conflict")
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	_ = s.Log(ctx, approvedBy, "logs.purge.executed", "request_id="+int64ToString(requestID)+" "+details)
	return &AuditPurgeResult{RequestID: requestID, Deleted: deleted}, nil
}

func int64ToString(v int64) string { return strconv.FormatInt(v, 10) }

func (s *auditStore) ListFiltered(ctx context.Context, since time.Time, limit int) ([]AuditRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, action, details, created_at FROM audit_log WHERE created_at>=? ORDER BY created_at DESC LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []AuditRecord
	for rows.Next() {
		var r AuditRecord
		if err := rows.Scan(&r.ID, &r.Username, &r.Action, &r.Details, &r.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}
