package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type AppRuntimeStore interface {
	GetRuntimeSettings(ctx context.Context) (*AppRuntimeSettings, error)
	SaveRuntimeSettings(ctx context.Context, settings *AppRuntimeSettings) error
}

type AppRuntimeSettings struct {
	ID                  int64     `json:"id"`
	DeploymentMode      string    `json:"deployment_mode"`
	UpdateChecksEnabled bool      `json:"update_checks_enabled"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type appRuntimeStore struct {
	db *sql.DB
}

func NewAppRuntimeStore(db *sql.DB) AppRuntimeStore {
	return &appRuntimeStore{db: db}
}

func (s *appRuntimeStore) GetRuntimeSettings(ctx context.Context) (*AppRuntimeSettings, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, deployment_mode, update_checks_enabled, updated_at
		FROM app_runtime_settings ORDER BY id LIMIT 1`)
	var out AppRuntimeSettings
	var enabled int
	if err := row.Scan(&out.ID, &out.DeploymentMode, &enabled, &out.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	out.UpdateChecksEnabled = enabled == 1
	out.DeploymentMode = normalizeDeploymentMode(out.DeploymentMode)
	return &out, nil
}

func (s *appRuntimeStore) SaveRuntimeSettings(ctx context.Context, settings *AppRuntimeSettings) error {
	if settings == nil {
		return errors.New("missing runtime settings")
	}
	settings.DeploymentMode = normalizeDeploymentMode(settings.DeploymentMode)
	now := time.Now().UTC()
	enabled := 0
	if settings.UpdateChecksEnabled {
		enabled = 1
	}
	if settings.ID > 0 {
		_, err := s.db.ExecContext(ctx, `
			UPDATE app_runtime_settings
			SET deployment_mode=?, update_checks_enabled=?, updated_at=?
			WHERE id=?`,
			settings.DeploymentMode,
			enabled,
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
		INSERT INTO app_runtime_settings(deployment_mode, update_checks_enabled, updated_at)
		VALUES(?,?,?)`,
		settings.DeploymentMode,
		enabled,
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

func normalizeDeploymentMode(mode string) string {
	val := strings.ToLower(strings.TrimSpace(mode))
	if val == "home" {
		return "home"
	}
	return "enterprise"
}
