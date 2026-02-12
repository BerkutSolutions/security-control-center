package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type SessionStore interface {
	SaveSession(ctx context.Context, sess *SessionRecord) error
	GetSession(ctx context.Context, id string) (*SessionRecord, error)
	ListByUser(ctx context.Context, userID int64) ([]SessionRecord, error)
	ListAll(ctx context.Context) ([]SessionRecord, error)
	DeleteSession(ctx context.Context, id string, by string) error
	DeleteAllForUser(ctx context.Context, userID int64, by string) error
	UpdateActivity(ctx context.Context, id string, now time.Time, extendBy time.Duration) error
}

type sessionsStore struct {
	db *sql.DB
}

func NewSessionsStore(db *sql.DB) SessionStore {
	return &sessionsStore{db: db}
}

func (s *sessionsStore) SaveSession(ctx context.Context, sess *SessionRecord) error {
	rolesJSON, _ := json.Marshal(sess.Roles)
	now := time.Now().UTC()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	if sess.LastSeenAt.IsZero() {
		sess.LastSeenAt = sess.CreatedAt
	}
	revokedAt := interface{}(nil)
	if sess.RevokedAt != nil {
		revokedAt = *sess.RevokedAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO sessions(id, user_id, username, roles, csrf_token, ip, user_agent, created_at, last_seen_at, expires_at, revoked, revoked_at, revoked_by) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sess.ID, sess.UserID, sess.Username, string(rolesJSON), sess.CSRFToken, sess.IP, sess.UserAgent, sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt, boolToInt(sess.Revoked), revokedAt, sess.RevokedBy)
	return err
}

func (s *sessionsStore) GetSession(ctx context.Context, id string) (*SessionRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, user_id, username, roles, csrf_token, ip, user_agent, created_at, last_seen_at, expires_at, revoked, revoked_at, revoked_by FROM sessions WHERE id=?`, id)
	var sr SessionRecord
	var rolesStr string
	var revoked int
	var revokedAt sql.NullTime
	if err := row.Scan(&sr.ID, &sr.UserID, &sr.Username, &rolesStr, &sr.CSRFToken, &sr.IP, &sr.UserAgent, &sr.CreatedAt, &sr.LastSeenAt, &sr.ExpiresAt, &revoked, &revokedAt, &sr.RevokedBy); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	sr.Revoked = revoked == 1
	if revokedAt.Valid {
		sr.RevokedAt = &revokedAt.Time
	}
	if sr.LastSeenAt.IsZero() {
		sr.LastSeenAt = sr.CreatedAt
	}
	if sr.Revoked {
		return nil, nil
	}
	if time.Now().After(sr.ExpiresAt) {
		_ = s.DeleteSession(ctx, id, "system")
		return nil, nil
	}
	_ = json.Unmarshal([]byte(rolesStr), &sr.Roles)
	return &sr, nil
}

func (s *sessionsStore) DeleteSession(ctx context.Context, id string, by string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked=1, revoked_at=?, revoked_by=?, expires_at=? WHERE id=?`, now, by, now, id)
	return err
}

func (s *sessionsStore) DeleteAllForUser(ctx context.Context, userID int64, by string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked=1, revoked_at=?, revoked_by=?, expires_at=? WHERE user_id=?`, now, by, now, userID)
	return err
}

func (s *sessionsStore) UpdateActivity(ctx context.Context, id string, now time.Time, extendBy time.Duration) error {
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at=?, expires_at=? WHERE id=? AND revoked=0`, now, now.Add(extendBy), id)
	return err
}

func (s *sessionsStore) ListByUser(ctx context.Context, userID int64) ([]SessionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, username, roles, csrf_token, ip, user_agent, created_at, last_seen_at, expires_at, revoked, revoked_at, revoked_by FROM sessions WHERE user_id=? AND revoked=0 AND expires_at > ? ORDER BY last_seen_at DESC`, userID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SessionRecord
	for rows.Next() {
		var sr SessionRecord
		var rolesStr string
		var revoked int
		var revokedAt sql.NullTime
		if err := rows.Scan(&sr.ID, &sr.UserID, &sr.Username, &rolesStr, &sr.CSRFToken, &sr.IP, &sr.UserAgent, &sr.CreatedAt, &sr.LastSeenAt, &sr.ExpiresAt, &revoked, &revokedAt, &sr.RevokedBy); err != nil {
			return nil, err
		}
		sr.Revoked = revoked == 1
		if revokedAt.Valid {
			sr.RevokedAt = &revokedAt.Time
		}
		if sr.LastSeenAt.IsZero() {
			sr.LastSeenAt = sr.CreatedAt
		}
		_ = json.Unmarshal([]byte(rolesStr), &sr.Roles)
		res = append(res, sr)
	}
	return res, rows.Err()
}

func (s *sessionsStore) ListAll(ctx context.Context) ([]SessionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, username, roles, csrf_token, ip, user_agent, created_at, last_seen_at, expires_at, revoked, revoked_at, revoked_by FROM sessions WHERE revoked=0 AND expires_at > ?`, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []SessionRecord
	for rows.Next() {
		var sr SessionRecord
		var rolesStr string
		var revoked int
		var revokedAt sql.NullTime
		if err := rows.Scan(&sr.ID, &sr.UserID, &sr.Username, &rolesStr, &sr.CSRFToken, &sr.IP, &sr.UserAgent, &sr.CreatedAt, &sr.LastSeenAt, &sr.ExpiresAt, &revoked, &revokedAt, &sr.RevokedBy); err != nil {
			return nil, err
		}
		sr.Revoked = revoked == 1
		if revokedAt.Valid {
			sr.RevokedAt = &revokedAt.Time
		}
		if sr.LastSeenAt.IsZero() {
			sr.LastSeenAt = sr.CreatedAt
		}
		_ = json.Unmarshal([]byte(rolesStr), &sr.Roles)
		res = append(res, sr)
	}
	return res, rows.Err()
}
