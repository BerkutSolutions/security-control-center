package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

var (
	ErrControlTypeExists  = errors.New("control type exists")
	ErrControlTypeBuiltin = errors.New("control type is builtin")
	ErrControlTypeInUse   = errors.New("control type in use")
)

type ControlType struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsBuiltin bool      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *controlsStore) ListControlTypes(ctx context.Context) ([]ControlType, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, is_builtin, created_at FROM control_types ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlType
	for rows.Next() {
		var item ControlType
		var builtin int
		if err := rows.Scan(&item.ID, &item.Name, &builtin, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.IsBuiltin = builtin == 1
		res = append(res, item)
	}
	return res, rows.Err()
}

func (s *controlsStore) GetControlTypeByID(ctx context.Context, id int64) (*ControlType, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, is_builtin, created_at FROM control_types WHERE id=?`, id)
	var item ControlType
	var builtin int
	if err := row.Scan(&item.ID, &item.Name, &builtin, &item.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.IsBuiltin = builtin == 1
	return &item, nil
}

func (s *controlsStore) GetControlTypeByName(ctx context.Context, name string) (*ControlType, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, name, is_builtin, created_at FROM control_types WHERE lower(name)=?`, strings.ToLower(normalized))
	var item ControlType
	var builtin int
	if err := row.Scan(&item.ID, &item.Name, &builtin, &item.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.IsBuiltin = builtin == 1
	return &item, nil
}

func (s *controlsStore) CreateControlType(ctx context.Context, name string, isBuiltin bool) (*ControlType, error) {
	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return nil, errors.New("name required")
	}
	existing, err := s.GetControlTypeByName(ctx, normalized)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrControlTypeExists
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO control_types(name, is_builtin, created_at) VALUES(?,?,?)`, normalized, boolToInt(isBuiltin), now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ControlType{ID: id, Name: normalized, IsBuiltin: isBuiltin, CreatedAt: now}, nil
}

func (s *controlsStore) DeleteControlType(ctx context.Context, id int64) error {
	item, err := s.GetControlTypeByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return sql.ErrNoRows
	}
	if item.IsBuiltin {
		return ErrControlTypeBuiltin
	}
	var used int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM controls WHERE lower(control_type)=?`, strings.ToLower(item.Name)).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return ErrControlTypeInUse
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM control_types WHERE id=?`, id)
	return err
}
