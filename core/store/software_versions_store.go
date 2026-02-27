package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *softwareStore) ListVersions(ctx context.Context, productID int64, includeDeleted bool) ([]SoftwareVersion, error) {
	if productID <= 0 {
		return nil, errors.New("bad product id")
	}
	clauses := []string{"product_id=?"}
	args := []any{productID}
	if !includeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	query := `
		SELECT id, product_id, version, release_date, eol_date, notes, created_by, updated_by, created_at, updated_at, deleted_at
		FROM software_versions
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY deleted_at IS NOT NULL, LOWER(version) ASC, id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SoftwareVersion
	for rows.Next() {
		item, err := scanSoftwareVersionRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *softwareStore) CreateVersion(ctx context.Context, v *SoftwareVersion) (int64, error) {
	if v == nil || v.ProductID <= 0 {
		return 0, errors.New("bad version")
	}
	now := time.Now().UTC()
	v.Version = strings.TrimSpace(v.Version)
	v.Notes = strings.TrimSpace(v.Notes)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO software_versions(product_id, version, release_date, eol_date, notes, created_by, updated_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)
	`, v.ProductID, v.Version, nullableTime(v.ReleaseDate), nullableTime(v.EOLDate), v.Notes, nullableID(v.CreatedBy), nullableID(v.UpdatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *softwareStore) UpdateVersion(ctx context.Context, v *SoftwareVersion) error {
	if v == nil || v.ID <= 0 {
		return errors.New("bad id")
	}
	now := time.Now().UTC()
	v.Version = strings.TrimSpace(v.Version)
	v.Notes = strings.TrimSpace(v.Notes)
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_versions
		SET version=?, release_date=?, eol_date=?, notes=?, updated_by=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL
	`, v.Version, nullableTime(v.ReleaseDate), nullableTime(v.EOLDate), v.Notes, nullableID(v.UpdatedBy), now, v.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *softwareStore) ArchiveVersion(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_versions SET deleted_at=?, updated_by=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL
	`, now, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *softwareStore) RestoreVersion(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_versions SET deleted_at=NULL, updated_by=?, updated_at=?
		WHERE id=? AND deleted_at IS NOT NULL
	`, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type softwareVersionRowScanner interface {
	Scan(dest ...any) error
}

func scanSoftwareVersionRow(rows softwareVersionRowScanner) (*SoftwareVersion, error) {
	var v SoftwareVersion
	var release, eol, deleted sql.NullTime
	var createdBy, updatedBy sql.NullInt64
	if err := rows.Scan(&v.ID, &v.ProductID, &v.Version, &release, &eol, &v.Notes, &createdBy, &updatedBy, &v.CreatedAt, &v.UpdatedAt, &deleted); err != nil {
		return nil, err
	}
	if release.Valid {
		t := release.Time
		v.ReleaseDate = &t
	}
	if eol.Valid {
		t := eol.Time
		v.EOLDate = &t
	}
	if deleted.Valid {
		t := deleted.Time
		v.DeletedAt = &t
	}
	if createdBy.Valid {
		x := createdBy.Int64
		v.CreatedBy = &x
	}
	if updatedBy.Valid {
		x := updatedBy.Int64
		v.UpdatedBy = &x
	}
	return &v, nil
}
