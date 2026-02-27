package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLStore) CreateSpace(ctx context.Context, space *tasks.Space, acl []tasks.ACLRule) (int64, error) {
	now := time.Now().UTC()
	layout := strings.TrimSpace(space.Layout)
	if layout == "" {
		layout = "row"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_spaces(organization_id, name, description, layout, created_by, created_at, updated_at, is_active)
		VALUES(?,?,?,?,?,?,?,?)`,
		strings.TrimSpace(space.OrganizationID), space.Name, space.Description, layout, nullableID(space.CreatedBy), now, now, boolToInt(space.IsActive))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	space.ID = id
	space.CreatedAt = now
	space.UpdatedAt = now
	space.Layout = layout
	if len(acl) == 0 {
		return id, nil
	}
	if err := s.SetSpaceACL(ctx, id, acl); err != nil {
		return id, err
	}
	return id, nil
}

func (s *SQLStore) UpdateSpace(ctx context.Context, space *tasks.Space) error {
	layout := strings.TrimSpace(space.Layout)
	if layout == "" {
		layout = "row"
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE task_spaces SET organization_id=?, name=?, description=?, layout=?, is_active=?, updated_at=?
		WHERE id=?`,
		strings.TrimSpace(space.OrganizationID), space.Name, space.Description, layout, boolToInt(space.IsActive), time.Now().UTC(), space.ID)
	return err
}

func (s *SQLStore) DeleteSpace(ctx context.Context, spaceID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE task_spaces SET is_active=0, updated_at=? WHERE id=?`, time.Now().UTC(), spaceID)
	return err
}

func (s *SQLStore) GetSpace(ctx context.Context, spaceID int64) (*tasks.Space, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, name, description, layout, created_by, created_at, updated_at, is_active
		FROM task_spaces WHERE id=?`, spaceID)
	var sp tasks.Space
	var createdBy sql.NullInt64
	var active int
	if err := row.Scan(&sp.ID, &sp.OrganizationID, &sp.Name, &sp.Description, &sp.Layout, &createdBy, &sp.CreatedAt, &sp.UpdatedAt, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if createdBy.Valid {
		sp.CreatedBy = &createdBy.Int64
	}
	sp.IsActive = active == 1
	return &sp, nil
}

func (s *SQLStore) ListSpaces(ctx context.Context, filter tasks.SpaceFilter) ([]tasks.Space, error) {
	query := `
		SELECT id, organization_id, name, description, layout, created_by, created_at, updated_at, is_active
		FROM task_spaces`
	if !filter.IncludeInactive {
		query += " WHERE is_active=1"
	}
	query += " ORDER BY updated_at DESC"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Space
	for rows.Next() {
		var sp tasks.Space
		var createdBy sql.NullInt64
		var active int
		if err := rows.Scan(&sp.ID, &sp.OrganizationID, &sp.Name, &sp.Description, &sp.Layout, &createdBy, &sp.CreatedAt, &sp.UpdatedAt, &active); err != nil {
			return nil, err
		}
		if createdBy.Valid {
			sp.CreatedBy = &createdBy.Int64
		}
		sp.IsActive = active == 1
		res = append(res, sp)
	}
	return res, rows.Err()
}

func (s *SQLStore) SetSpaceACL(ctx context.Context, spaceID int64, acl []tasks.ACLRule) error {
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_space_acl WHERE space_id=?`, spaceID); err != nil {
			return err
		}
		for _, a := range acl {
			if _, err := tx.ExecContext(ctx, `INSERT INTO task_space_acl(space_id, subject_type, subject_id, permission) VALUES(?,?,?,?)`,
				spaceID, strings.ToLower(a.SubjectType), a.SubjectID, strings.ToLower(a.Permission)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SQLStore) GetSpaceACL(ctx context.Context, spaceID int64) ([]tasks.ACLRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT subject_type, subject_id, permission FROM task_space_acl WHERE space_id=?`, spaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.ACLRule
	for rows.Next() {
		var a tasks.ACLRule
		if err := rows.Scan(&a.SubjectType, &a.SubjectID, &a.Permission); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}
