package store

import (
	"context"
	"database/sql"
	"time"
)

type AuditStore interface {
	Log(ctx context.Context, username, action, details string) error
	List(ctx context.Context) ([]AuditRecord, error)
	ListFiltered(ctx context.Context, since time.Time, limit int) ([]AuditRecord, error)
}

type AuditRecord struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type auditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) AuditStore {
	return &auditStore{db: db}
}

func (s *auditStore) Log(ctx context.Context, username, action, details string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_log(username, action, details, created_at) VALUES(?,?,?,?)`, username, action, details, time.Now().UTC())
	return err
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
