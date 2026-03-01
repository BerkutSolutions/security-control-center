package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PasskeyRecord struct {
	ID              int64      `json:"id"`
	UserID          int64      `json:"user_id"`
	Name            string     `json:"name"`
	CredentialID    string     `json:"credential_id"`
	PublicKey       []byte     `json:"-"`
	AttestationType string     `json:"attestation_type"`
	TransportsJSON  string     `json:"transports_json"`
	AAGUID          []byte     `json:"-"`
	SignCount       int64      `json:"sign_count"`
	CreatedAt       time.Time  `json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
}

type WebAuthnChallenge struct {
	ID              string
	Kind            string
	UserID          *int64
	SessionDataJSON string
	IP              string
	UserAgent       string
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

type PasskeysStore interface {
	ListUserPasskeys(ctx context.Context, userID int64) ([]PasskeyRecord, error)
	GetPasskeyByID(ctx context.Context, id int64) (*PasskeyRecord, error)
	GetPasskeyByCredentialID(ctx context.Context, credentialID string) (*PasskeyRecord, error)
	CreatePasskey(ctx context.Context, rec *PasskeyRecord) (int64, error)
	UpdatePasskeyUsage(ctx context.Context, id int64, signCount int64, now time.Time) error
	RenamePasskey(ctx context.Context, id int64, name string) error
	DeletePasskey(ctx context.Context, id int64) error

	CreateChallenge(ctx context.Context, kind string, userID *int64, sessionData any, ip, userAgent string, expiresAt time.Time) (string, error)
	GetChallenge(ctx context.Context, id string) (*WebAuthnChallenge, error)
	DeleteChallenge(ctx context.Context, id string) error
	DeleteExpiredChallenges(ctx context.Context, now time.Time) error
}

type passkeysStore struct {
	db *sql.DB
}

func NewPasskeysStore(db *sql.DB) PasskeysStore {
	return &passkeysStore{db: db}
}

func (s *passkeysStore) ListUserPasskeys(ctx context.Context, userID int64) ([]PasskeyRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, credential_id, public_key, attestation_type, transports_json, aaguid, sign_count, created_at, last_used_at
		FROM user_passkeys
		WHERE user_id=?
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PasskeyRecord{}
	for rows.Next() {
		var r PasskeyRecord
		var aaguid []byte
		var last sql.NullTime
		if err := rows.Scan(&r.ID, &r.UserID, &r.Name, &r.CredentialID, &r.PublicKey, &r.AttestationType, &r.TransportsJSON, &aaguid, &r.SignCount, &r.CreatedAt, &last); err != nil {
			return nil, err
		}
		r.AAGUID = aaguid
		if last.Valid {
			t := last.Time.UTC()
			r.LastUsedAt = &t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *passkeysStore) GetPasskeyByID(ctx context.Context, id int64) (*PasskeyRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, credential_id, public_key, attestation_type, transports_json, aaguid, sign_count, created_at, last_used_at
		FROM user_passkeys WHERE id=?`, id)
	var r PasskeyRecord
	var aaguid []byte
	var last sql.NullTime
	if err := row.Scan(&r.ID, &r.UserID, &r.Name, &r.CredentialID, &r.PublicKey, &r.AttestationType, &r.TransportsJSON, &aaguid, &r.SignCount, &r.CreatedAt, &last); err != nil {
		return nil, err
	}
	r.AAGUID = aaguid
	if last.Valid {
		t := last.Time.UTC()
		r.LastUsedAt = &t
	}
	return &r, nil
}

func (s *passkeysStore) GetPasskeyByCredentialID(ctx context.Context, credentialID string) (*PasskeyRecord, error) {
	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return nil, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, credential_id, public_key, attestation_type, transports_json, aaguid, sign_count, created_at, last_used_at
		FROM user_passkeys WHERE credential_id=?`, credentialID)
	var r PasskeyRecord
	var aaguid []byte
	var last sql.NullTime
	if err := row.Scan(&r.ID, &r.UserID, &r.Name, &r.CredentialID, &r.PublicKey, &r.AttestationType, &r.TransportsJSON, &aaguid, &r.SignCount, &r.CreatedAt, &last); err != nil {
		return nil, err
	}
	r.AAGUID = aaguid
	if last.Valid {
		t := last.Time.UTC()
		r.LastUsedAt = &t
	}
	return &r, nil
}

func (s *passkeysStore) CreatePasskey(ctx context.Context, rec *PasskeyRecord) (int64, error) {
	if rec == nil {
		return 0, fmt.Errorf("nil passkey")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO user_passkeys(user_id, name, credential_id, public_key, attestation_type, transports_json, aaguid, sign_count, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, rec.UserID, strings.TrimSpace(rec.Name), strings.TrimSpace(rec.CredentialID), rec.PublicKey, strings.TrimSpace(rec.AttestationType), strings.TrimSpace(rec.TransportsJSON), rec.AAGUID, rec.SignCount, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *passkeysStore) UpdatePasskeyUsage(ctx context.Context, id int64, signCount int64, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_passkeys SET sign_count=?, last_used_at=? WHERE id=?`, signCount, now.UTC(), id)
	return err
}

func (s *passkeysStore) RenamePasskey(ctx context.Context, id int64, name string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_passkeys SET name=? WHERE id=?`, strings.TrimSpace(name), id)
	return err
}

func (s *passkeysStore) DeletePasskey(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_passkeys WHERE id=?`, id)
	return err
}

func randomToken(n int) (string, error) {
	if n <= 0 {
		n = 32
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *passkeysStore) CreateChallenge(ctx context.Context, kind string, userID *int64, sessionData any, ip, userAgent string, expiresAt time.Time) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("db missing")
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "login"
	}
	raw, err := json.Marshal(sessionData)
	if err != nil {
		return "", err
	}
	id, err := randomToken(32)
	if err != nil {
		return "", err
	}
	var uid any = nil
	if userID != nil && *userID > 0 {
		uid = *userID
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO webauthn_challenges(id, kind, user_id, session_data_json, ip, user_agent, expires_at, created_at)
		VALUES(?,?,?,?,?,?,?,?)
	`, id, kind, uid, string(raw), strings.TrimSpace(ip), strings.TrimSpace(userAgent), expiresAt.UTC(), time.Now().UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *passkeysStore) GetChallenge(ctx context.Context, id string) (*WebAuthnChallenge, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, kind, user_id, session_data_json, ip, user_agent, expires_at, created_at
		FROM webauthn_challenges WHERE id=?`, id)
	var c WebAuthnChallenge
	var uid sql.NullInt64
	if err := row.Scan(&c.ID, &c.Kind, &uid, &c.SessionDataJSON, &c.IP, &c.UserAgent, &c.ExpiresAt, &c.CreatedAt); err != nil {
		return nil, err
	}
	if uid.Valid {
		v := uid.Int64
		c.UserID = &v
	}
	return &c, nil
}

func (s *passkeysStore) DeleteChallenge(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webauthn_challenges WHERE id=?`, strings.TrimSpace(id))
	return err
}

func (s *passkeysStore) DeleteExpiredChallenges(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webauthn_challenges WHERE expires_at<?`, now.UTC())
	return err
}
