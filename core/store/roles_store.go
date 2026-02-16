package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type RolesStore interface {
	List(ctx context.Context) ([]Role, error)
	FindByName(ctx context.Context, name string) (*Role, error)
	FindByID(ctx context.Context, id int64) (*Role, error)
	Create(ctx context.Context, role *Role) (int64, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id int64) error
	EnsureBuiltIn(ctx context.Context, roles []Role) error
}

type rolesStore struct {
	db *sql.DB
}

func NewRolesStore(db *sql.DB) RolesStore {
	return &rolesStore{db: db}
}

func (s *rolesStore) List(ctx context.Context) ([]Role, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, permissions, built_in, template, created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Role
	for rows.Next() {
		var r Role
		var permsRaw string
		var builtIn, tmpl int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &permsRaw, &builtIn, &tmpl, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(permsRaw), &r.Permissions)
		r.BuiltIn = builtIn == 1
		r.Template = tmpl == 1
		res = append(res, r)
	}
	return res, rows.Err()
}

func (s *rolesStore) FindByName(ctx context.Context, name string) (*Role, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, permissions, built_in, template, created_at, updated_at FROM roles WHERE name=?`, strings.ToLower(name))
	var r Role
	var permsRaw string
	var builtIn, tmpl int
	if err := row.Scan(&r.ID, &r.Name, &r.Description, &permsRaw, &builtIn, &tmpl, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(permsRaw), &r.Permissions)
	r.BuiltIn = builtIn == 1
	r.Template = tmpl == 1
	return &r, nil
}

func (s *rolesStore) Create(ctx context.Context, role *Role) (int64, error) {
	role.Name = strings.ToLower(role.Name)
	permsJSON, _ := json.Marshal(role.Permissions)
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO roles(name, description, permissions, built_in, template, created_at, updated_at) VALUES(?,?,?,?,?, ?, ?)`,
		role.Name, role.Description, string(permsJSON), boolToInt(role.BuiltIn), boolToInt(role.Template), now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *rolesStore) Update(ctx context.Context, role *Role) error {
	role.Name = strings.ToLower(role.Name)
	permsJSON, _ := json.Marshal(role.Permissions)
	res, err := s.db.ExecContext(ctx, `UPDATE roles SET description=?, permissions=?, template=?, updated_at=? WHERE id=? AND built_in=0`,
		role.Description, string(permsJSON), boolToInt(role.Template), time.Now().UTC(), role.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *rolesStore) Delete(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE id=? AND built_in=0`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *rolesStore) EnsureBuiltIn(ctx context.Context, roles []Role) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, r := range roles {
		r.Name = strings.ToLower(r.Name)
		permsJSON, _ := json.Marshal(r.Permissions)
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name=?`, r.Name).Scan(&id)
		if err != nil {
			if err == sql.ErrNoRows {
				if _, err := tx.ExecContext(ctx, `INSERT INTO roles(name, description, permissions, built_in, template, created_at, updated_at) VALUES(?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
					r.Name, r.Description, string(permsJSON), 1, boolToInt(r.Template)); err != nil {
					tx.Rollback()
					return err
				}
				continue
			}
			tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE roles SET description=?, permissions=?, built_in=1, template=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			r.Description, string(permsJSON), boolToInt(r.Template), id); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *rolesStore) FindByID(ctx context.Context, id int64) (*Role, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, permissions, built_in, template, created_at, updated_at FROM roles WHERE id=?`, id)
	var r Role
	var permsRaw string
	var builtIn, tmpl int
	if err := row.Scan(&r.ID, &r.Name, &r.Description, &permsRaw, &builtIn, &tmpl, &r.CreatedAt, &r.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal([]byte(permsRaw), &r.Permissions)
	r.BuiltIn = builtIn == 1
	r.Template = tmpl == 1
	return &r, nil
}
