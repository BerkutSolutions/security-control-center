package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type AppHTTPSStore interface {
	GetHTTPSSettings(ctx context.Context) (*HTTPSSettings, error)
	SaveHTTPSSettings(ctx context.Context, settings *HTTPSSettings) error
}

type HTTPSSettings struct {
	ID                int64     `json:"id"`
	Mode              string    `json:"mode"`
	ListenPort        int       `json:"listen_port"`
	TrustedProxies    []string  `json:"trusted_proxies"`
	BuiltinCertPath   string    `json:"builtin_cert_path"`
	BuiltinKeyPath    string    `json:"builtin_key_path"`
	ExternalProxyHint string    `json:"external_proxy_hint"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type appHTTPSStore struct {
	db *sql.DB
}

func NewAppHTTPSStore(db *sql.DB) AppHTTPSStore {
	return &appHTTPSStore{db: db}
}

func (s *appHTTPSStore) GetHTTPSSettings(ctx context.Context) (*HTTPSSettings, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, mode, listen_port, trusted_proxies_json, builtin_cert_path, builtin_key_path, external_proxy_hint, updated_at
		FROM app_https_settings ORDER BY id LIMIT 1`)
	var out HTTPSSettings
	var trustedRaw string
	if err := row.Scan(
		&out.ID,
		&out.Mode,
		&out.ListenPort,
		&trustedRaw,
		&out.BuiltinCertPath,
		&out.BuiltinKeyPath,
		&out.ExternalProxyHint,
		&out.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(trustedRaw) != "" {
		_ = json.Unmarshal([]byte(trustedRaw), &out.TrustedProxies)
	}
	return &out, nil
}

func (s *appHTTPSStore) SaveHTTPSSettings(ctx context.Context, settings *HTTPSSettings) error {
	if settings == nil {
		return errors.New("missing https settings")
	}
	trustedJSON, _ := json.Marshal(settings.TrustedProxies)
	now := time.Now().UTC()
	if settings.ID > 0 {
		_, err := s.db.ExecContext(ctx, `
			UPDATE app_https_settings
			SET mode=?, listen_port=?, trusted_proxies_json=?, builtin_cert_path=?, builtin_key_path=?, external_proxy_hint=?, updated_at=?
			WHERE id=?`,
			settings.Mode,
			settings.ListenPort,
			string(trustedJSON),
			settings.BuiltinCertPath,
			settings.BuiltinKeyPath,
			settings.ExternalProxyHint,
			now,
			settings.ID,
		)
		if err != nil {
			return err
		}
		settings.UpdatedAt = now
		return nil
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO app_https_settings(mode, listen_port, trusted_proxies_json, builtin_cert_path, builtin_key_path, external_proxy_hint, updated_at)
		VALUES(?,?,?,?,?,?,?)`,
		settings.Mode,
		settings.ListenPort,
		string(trustedJSON),
		settings.BuiltinCertPath,
		settings.BuiltinKeyPath,
		settings.ExternalProxyHint,
		now,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	settings.ID = id
	settings.UpdatedAt = now
	return nil
}
