package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *softwareStore) ListProducts(ctx context.Context, filter SoftwareFilter) ([]SoftwareProduct, error) {
	clauses := []string{}
	args := []any{}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		clauses = append(clauses, "(LOWER(name) LIKE ? OR LOWER(vendor) LIKE ? OR LOWER(description) LIKE ?)")
		p := "%" + strings.ToLower(q) + "%"
		args = append(args, p, p, p)
	}
	if v := strings.TrimSpace(filter.Vendor); v != "" {
		clauses = append(clauses, "LOWER(vendor)=?")
		args = append(args, strings.ToLower(v))
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		clauses = append(clauses, "tags_json LIKE ?")
		args = append(args, "%"+strings.ToUpper(tag)+"%")
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := `SELECT id, name, vendor, description, tags_json, created_by, updated_by, created_at, updated_at, version, deleted_at FROM software_products`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY deleted_at IS NOT NULL, LOWER(name) ASC, id ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SoftwareProduct
	for rows.Next() {
		p, err := scanSoftwareProductRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *softwareStore) ListProductsLite(ctx context.Context, search string, limit int) ([]SoftwareProductLite, error) {
	clauses := []string{"deleted_at IS NULL"}
	args := []any{}
	if q := strings.TrimSpace(search); q != "" {
		clauses = append(clauses, "(LOWER(name) LIKE ? OR LOWER(vendor) LIKE ?)")
		p := "%" + strings.ToLower(q) + "%"
		args = append(args, p, p)
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, name, vendor FROM software_products WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY LOWER(name) ASC, id ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SoftwareProductLite
	for rows.Next() {
		var p SoftwareProductLite
		if err := rows.Scan(&p.ID, &p.Name, &p.Vendor); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *softwareStore) GetProduct(ctx context.Context, id int64) (*SoftwareProduct, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, vendor, description, tags_json, created_by, updated_by, created_at, updated_at, version, deleted_at FROM software_products WHERE id=?`, id)
	return scanSoftwareProduct(row)
}

func (s *softwareStore) CreateProduct(ctx context.Context, p *SoftwareProduct) (int64, error) {
	if p == nil {
		return 0, errors.New("nil product")
	}
	now := time.Now().UTC()
	p.Name = strings.TrimSpace(p.Name)
	p.Vendor = strings.TrimSpace(p.Vendor)
	p.Description = strings.TrimSpace(p.Description)
	p.Tags = normalizeTags(p.Tags)
	tagsJSON, _ := json.Marshal(normalizeUpperTags(p.Tags))
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO software_products(name, vendor, description, tags_json, created_by, updated_by, created_at, updated_at, version)
		VALUES(?,?,?,?,?,?,?,?,1)
	`, p.Name, p.Vendor, p.Description, string(tagsJSON), nullableID(p.CreatedBy), nullableID(p.UpdatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *softwareStore) UpdateProduct(ctx context.Context, p *SoftwareProduct) error {
	if p == nil || p.ID <= 0 {
		return errors.New("invalid product")
	}
	now := time.Now().UTC()
	p.Name = strings.TrimSpace(p.Name)
	p.Vendor = strings.TrimSpace(p.Vendor)
	p.Description = strings.TrimSpace(p.Description)
	p.Tags = normalizeTags(p.Tags)
	tagsJSON, _ := json.Marshal(normalizeUpperTags(p.Tags))
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_products
		SET name=?, vendor=?, description=?, tags_json=?, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND version=? AND deleted_at IS NULL
	`, p.Name, p.Vendor, p.Description, string(tagsJSON), nullableID(p.UpdatedBy), now, p.ID, p.Version)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func (s *softwareStore) ArchiveProduct(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_products SET deleted_at=?, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND deleted_at IS NULL
	`, now, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func (s *softwareStore) RestoreProduct(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE software_products SET deleted_at=NULL, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND deleted_at IS NOT NULL
	`, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

type softwareProductRowScanner interface {
	Scan(dest ...any) error
}

func scanSoftwareProduct(row *sql.Row) (*SoftwareProduct, error) {
	return scanSoftwareProductRow(row)
}

func scanSoftwareProductRow(rows softwareProductRowScanner) (*SoftwareProduct, error) {
	var p SoftwareProduct
	var tagsRaw string
	var createdBy, updatedBy sql.NullInt64
	if err := rows.Scan(&p.ID, &p.Name, &p.Vendor, &p.Description, &tagsRaw, &createdBy, &updatedBy, &p.CreatedAt, &p.UpdatedAt, &p.Version, &p.DeletedAt); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		v := createdBy.Int64
		p.CreatedBy = &v
	}
	if updatedBy.Valid {
		v := updatedBy.Int64
		p.UpdatedBy = &v
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &p.Tags)
	}
	return &p, nil
}
