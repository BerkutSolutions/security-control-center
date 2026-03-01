package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type AppModuleState struct {
	ModuleID              string     `json:"module_id"`
	AppliedSchemaVersion  int        `json:"applied_schema_version"`
	AppliedBehaviorVersion int       `json:"applied_behavior_version"`
	InitializedAt         *time.Time `json:"initialized_at,omitempty"`
	UpdatedAt             time.Time  `json:"updated_at"`
	LastError             string     `json:"last_error,omitempty"`
}

type AppModuleStateStore interface {
	Get(ctx context.Context, moduleID string) (*AppModuleState, error)
	List(ctx context.Context) ([]AppModuleState, error)
	Upsert(ctx context.Context, st *AppModuleState) error
}

type appModuleStateStore struct {
	db *sql.DB
}

func NewAppModuleStateStore(db *sql.DB) AppModuleStateStore {
	return &appModuleStateStore{db: db}
}

func (s *appModuleStateStore) Get(ctx context.Context, moduleID string) (*AppModuleState, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil db")
	}
	id := strings.TrimSpace(moduleID)
	if id == "" {
		return nil, errors.New("empty module_id")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT module_id, applied_schema_version, applied_behavior_version, initialized_at, updated_at, last_error
		FROM app_module_state
		WHERE module_id=?`, id)
	return scanAppModuleState(row)
}

func (s *appModuleStateStore) List(ctx context.Context) ([]AppModuleState, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("nil db")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT module_id, applied_schema_version, applied_behavior_version, initialized_at, updated_at, last_error
		FROM app_module_state
		ORDER BY module_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppModuleState
	for rows.Next() {
		st, err := scanAppModuleState(rows)
		if err != nil {
			return nil, err
		}
		if st != nil {
			out = append(out, *st)
		}
	}
	return out, rows.Err()
}

func (s *appModuleStateStore) Upsert(ctx context.Context, st *AppModuleState) error {
	if s == nil || s.db == nil {
		return errors.New("nil db")
	}
	if st == nil {
		return errors.New("nil state")
	}
	id := strings.TrimSpace(st.ModuleID)
	if id == "" {
		return errors.New("empty module_id")
	}
	now := time.Now().UTC()
	initialized := st.InitializedAt
	if initialized != nil && initialized.IsZero() {
		initialized = nil
	}
	lastErr := strings.TrimSpace(st.LastError)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO app_module_state(module_id, applied_schema_version, applied_behavior_version, initialized_at, updated_at, last_error)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT (module_id)
		DO UPDATE SET
			applied_schema_version=excluded.applied_schema_version,
			applied_behavior_version=excluded.applied_behavior_version,
			initialized_at=excluded.initialized_at,
			updated_at=excluded.updated_at,
			last_error=excluded.last_error
	`, id, st.AppliedSchemaVersion, st.AppliedBehaviorVersion, initialized, now, lastErr)
	return err
}

func scanAppModuleState(row interface{ Scan(dest ...any) error }) (*AppModuleState, error) {
	var st AppModuleState
	var init sql.NullTime
	var updated sql.NullTime
	var lastErr sql.NullString
	if err := row.Scan(&st.ModuleID, &st.AppliedSchemaVersion, &st.AppliedBehaviorVersion, &init, &updated, &lastErr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if init.Valid {
		t := init.Time.UTC()
		st.InitializedAt = &t
	}
	if updated.Valid {
		st.UpdatedAt = updated.Time.UTC()
	} else {
		st.UpdatedAt = time.Now().UTC()
	}
	if lastErr.Valid {
		st.LastError = lastErr.String
	}
	return &st, nil
}

