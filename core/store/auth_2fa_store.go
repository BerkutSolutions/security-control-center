package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

type Auth2FAStore interface {
	UpsertTOTPSetup(ctx context.Context, userID int64, secretEnc string, expiresAt time.Time) error
	GetTOTPSetup(ctx context.Context, userID int64) (*TOTPSetupRecord, error)
	DeleteTOTPSetup(ctx context.Context, userID int64) error

	CreateChallenge(ctx context.Context, userID int64, ip, userAgent string, expiresAt time.Time) (string, error)
	GetChallenge(ctx context.Context, id string) (*TwoFAChallengeRecord, error)
	DeleteChallenge(ctx context.Context, id string) error
	DeleteChallengesForUser(ctx context.Context, userID int64) error
	DeleteExpiredChallenges(ctx context.Context, now time.Time) error

	DeleteRecoveryCodes(ctx context.Context, userID int64) error
	InsertRecoveryCodes(ctx context.Context, userID int64, items []RecoveryCodeHash) error
	ListUnusedRecoveryCodes(ctx context.Context, userID int64) ([]RecoveryCodeRecord, error)
	MarkRecoveryCodeUsed(ctx context.Context, id int64, usedAt time.Time, ip, userAgent string) error
	CountUnusedRecoveryCodes(ctx context.Context, userID int64) (int, error)
}

type TOTPSetupRecord struct {
	UserID    int64
	SecretEnc string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type TwoFAChallengeRecord struct {
	ID        string
	UserID    int64
	IP        string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type RecoveryCodeHash struct {
	Hash string
	Salt string
}

type RecoveryCodeRecord struct {
	ID        int64
	UserID    int64
	Hash      string
	Salt      string
	CreatedAt time.Time
	UsedAt    *time.Time
}

type auth2FAStore struct {
	db *sql.DB
}

func NewAuth2FAStore(db *sql.DB) Auth2FAStore {
	return &auth2FAStore{db: db}
}

func (s *auth2FAStore) UpsertTOTPSetup(ctx context.Context, userID int64, secretEnc string, expiresAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	secretEnc = strings.TrimSpace(secretEnc)
	if secretEnc == "" || userID <= 0 {
		return errors.New("invalid totp setup")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_totp_setup(user_id, secret_enc, expires_at)
		VALUES(?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET secret_enc=excluded.secret_enc, created_at=CURRENT_TIMESTAMP, expires_at=excluded.expires_at
	`, userID, secretEnc, expiresAt.UTC())
	return err
}

func (s *auth2FAStore) GetTOTPSetup(ctx context.Context, userID int64) (*TOTPSetupRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil db")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, secret_enc, created_at, expires_at
		FROM user_totp_setup
		WHERE user_id=?
	`, userID)
	var rec TOTPSetupRecord
	if err := row.Scan(&rec.UserID, &rec.SecretEnc, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (s *auth2FAStore) DeleteTOTPSetup(ctx context.Context, userID int64) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_totp_setup WHERE user_id=?`, userID)
	return err
}

func (s *auth2FAStore) CreateChallenge(ctx context.Context, userID int64, ip, userAgent string, expiresAt time.Time) (string, error) {
	if s == nil || s.db == nil {
		return "", errors.New("nil db")
	}
	id := strings.TrimSpace(newToken(24))
	if id == "" || userID <= 0 {
		return "", errors.New("invalid challenge")
	}
	ip = strings.TrimSpace(ip)
	userAgent = strings.TrimSpace(userAgent)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO auth_2fa_challenges(id, user_id, ip, user_agent, expires_at)
		VALUES(?, ?, ?, ?, ?)
	`, id, userID, ip, userAgent, expiresAt.UTC())
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *auth2FAStore) GetChallenge(ctx context.Context, id string) (*TwoFAChallengeRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil db")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, ip, user_agent, created_at, expires_at
		FROM auth_2fa_challenges
		WHERE id=?
	`, id)
	var rec TwoFAChallengeRecord
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.IP, &rec.UserAgent, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

func (s *auth2FAStore) DeleteChallenge(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_2fa_challenges WHERE id=?`, id)
	return err
}

func (s *auth2FAStore) DeleteChallengesForUser(ctx context.Context, userID int64) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	if userID <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_2fa_challenges WHERE user_id=?`, userID)
	return err
}

func (s *auth2FAStore) DeleteExpiredChallenges(ctx context.Context, now time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_2fa_challenges WHERE expires_at < ?`, now.UTC())
	return err
}

func (s *auth2FAStore) DeleteRecoveryCodes(ctx context.Context, userID int64) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	if userID <= 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_recovery_codes WHERE user_id=?`, userID)
	return err
}

func (s *auth2FAStore) InsertRecoveryCodes(ctx context.Context, userID int64, items []RecoveryCodeHash) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, it := range items {
		hash := strings.TrimSpace(it.Hash)
		salt := strings.TrimSpace(it.Salt)
		if hash == "" || salt == "" {
			_ = tx.Rollback()
			return errors.New("invalid recovery code hash")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_recovery_codes(user_id, code_hash, salt, created_at)
			VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		`, userID, hash, salt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *auth2FAStore) ListUnusedRecoveryCodes(ctx context.Context, userID int64) ([]RecoveryCodeRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil db")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, code_hash, salt, created_at, used_at
		FROM user_recovery_codes
		WHERE user_id=? AND used_at IS NULL
		ORDER BY id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RecoveryCodeRecord{}
	for rows.Next() {
		var rec RecoveryCodeRecord
		var used sql.NullTime
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.Hash, &rec.Salt, &rec.CreatedAt, &used); err != nil {
			return nil, err
		}
		if used.Valid {
			rec.UsedAt = &used.Time
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *auth2FAStore) MarkRecoveryCodeUsed(ctx context.Context, id int64, usedAt time.Time, ip, userAgent string) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	if id <= 0 {
		return errors.New("invalid id")
	}
	ip = strings.TrimSpace(ip)
	userAgent = strings.TrimSpace(userAgent)
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_recovery_codes
		SET used_at=?, used_ip=?, used_user_agent=?
		WHERE id=? AND used_at IS NULL
	`, usedAt.UTC(), ip, userAgent, id)
	return err
}

func (s *auth2FAStore) CountUnusedRecoveryCodes(ctx context.Context, userID int64) (int, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("nil db")
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM user_recovery_codes
		WHERE user_id=? AND used_at IS NULL
	`, userID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func newToken(n int) string {
	if n <= 0 {
		n = 24
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawStdEncoding.EncodeToString(buf)
}
