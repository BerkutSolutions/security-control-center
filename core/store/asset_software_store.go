package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *softwareStore) ListAssetSoftware(ctx context.Context, assetID int64, includeDeleted bool) ([]AssetSoftwareInstallation, error) {
	if assetID <= 0 {
		return nil, errors.New("bad asset id")
	}
	clauses := []string{"a.asset_id=?"}
	args := []any{assetID}
	if !includeDeleted {
		clauses = append(clauses, "a.deleted_at IS NULL")
	}
	query := `
		SELECT a.id, a.asset_id, a.product_id, a.version_id, a.version_text, a.installed_at, a.source, a.notes,
		       a.created_by, a.updated_by, a.created_at, a.updated_at, a.deleted_at,
		       p.name, p.vendor,
		       v.version
		FROM asset_software a
		JOIN software_products p ON p.id=a.product_id
		LEFT JOIN software_versions v ON v.id=a.version_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY a.deleted_at IS NOT NULL, LOWER(p.name) ASC, a.id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetSoftwareInstallation
	for rows.Next() {
		item, err := scanAssetSoftwareRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *softwareStore) AddAssetSoftware(ctx context.Context, inst *AssetSoftwareInstallation) (int64, error) {
	if inst == nil || inst.AssetID <= 0 || inst.ProductID <= 0 {
		return 0, errors.New("bad install")
	}
	now := time.Now().UTC()
	inst.Source = strings.ToLower(strings.TrimSpace(inst.Source))
	if inst.Source == "" {
		inst.Source = "manual"
	}
	inst.Notes = strings.TrimSpace(inst.Notes)
	inst.VersionText = strings.TrimSpace(inst.VersionText)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO asset_software(asset_id, product_id, version_id, version_text, installed_at, source, notes, created_by, updated_by, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`, inst.AssetID, inst.ProductID, nullableID(inst.VersionID), inst.VersionText, nullableTime(inst.InstalledAt), inst.Source, inst.Notes, nullableID(inst.CreatedBy), nullableID(inst.UpdatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *softwareStore) UpdateAssetSoftware(ctx context.Context, inst *AssetSoftwareInstallation) error {
	if inst == nil || inst.ID <= 0 {
		return errors.New("bad id")
	}
	now := time.Now().UTC()
	inst.Source = strings.ToLower(strings.TrimSpace(inst.Source))
	if inst.Source == "" {
		inst.Source = "manual"
	}
	inst.Notes = strings.TrimSpace(inst.Notes)
	inst.VersionText = strings.TrimSpace(inst.VersionText)
	res, err := s.db.ExecContext(ctx, `
		UPDATE asset_software
		SET version_id=?, version_text=?, installed_at=?, source=?, notes=?, updated_by=?, updated_at=?
		WHERE id=? AND deleted_at IS NULL
	`, nullableID(inst.VersionID), inst.VersionText, nullableTime(inst.InstalledAt), inst.Source, inst.Notes, nullableID(inst.UpdatedBy), now, inst.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *softwareStore) ArchiveAssetSoftware(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE asset_software SET deleted_at=?, updated_by=?, updated_at=?
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

func (s *softwareStore) RestoreAssetSoftware(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE asset_software SET deleted_at=NULL, updated_by=?, updated_at=?
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

func (s *softwareStore) ListProductAssets(ctx context.Context, productID int64, includeDeleted bool) ([]AssetSoftwareInstallation, error) {
	if productID <= 0 {
		return nil, errors.New("bad product id")
	}
	clauses := []string{"a.product_id=?"}
	args := []any{productID}
	if !includeDeleted {
		clauses = append(clauses, "a.deleted_at IS NULL")
	}
	query := `
		SELECT a.id, a.asset_id, a.product_id, a.version_id, a.version_text, a.installed_at, a.source, a.notes,
		       a.created_by, a.updated_by, a.created_at, a.updated_at, a.deleted_at,
		       p.name, p.vendor,
		       v.version,
		       s.name
		FROM asset_software a
		JOIN software_products p ON p.id=a.product_id
		LEFT JOIN software_versions v ON v.id=a.version_id
		JOIN assets s ON s.id=a.asset_id
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY a.deleted_at IS NOT NULL, LOWER(s.name) ASC, a.id ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetSoftwareInstallation
	for rows.Next() {
		var inst AssetSoftwareInstallation
		var vid sql.NullInt64
		var installed, deleted sql.NullTime
		var createdBy, updatedBy sql.NullInt64
		var versionLabel sql.NullString
		if err := rows.Scan(&inst.ID, &inst.AssetID, &inst.ProductID, &vid, &inst.VersionText, &installed, &inst.Source, &inst.Notes,
			&createdBy, &updatedBy, &inst.CreatedAt, &inst.UpdatedAt, &deleted,
			&inst.ProductName, &inst.ProductVendor, &versionLabel,
			&inst.AssetName,
		); err != nil {
			return nil, err
		}
		if vid.Valid {
			v := vid.Int64
			inst.VersionID = &v
		}
		if installed.Valid {
			t := installed.Time
			inst.InstalledAt = &t
		}
		if deleted.Valid {
			t := deleted.Time
			inst.DeletedAt = &t
		}
		if createdBy.Valid {
			v := createdBy.Int64
			inst.CreatedBy = &v
		}
		if updatedBy.Valid {
			v := updatedBy.Int64
			inst.UpdatedBy = &v
		}
		if versionLabel.Valid && strings.TrimSpace(versionLabel.String) != "" {
			inst.VersionLabel = versionLabel.String
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

type assetSoftwareRowScanner interface {
	Scan(dest ...any) error
}

func scanAssetSoftwareRow(rows assetSoftwareRowScanner) (*AssetSoftwareInstallation, error) {
	var inst AssetSoftwareInstallation
	var vid sql.NullInt64
	var installed, deleted sql.NullTime
	var createdBy, updatedBy sql.NullInt64
	var versionLabel sql.NullString
	if err := rows.Scan(&inst.ID, &inst.AssetID, &inst.ProductID, &vid, &inst.VersionText, &installed, &inst.Source, &inst.Notes,
		&createdBy, &updatedBy, &inst.CreatedAt, &inst.UpdatedAt, &deleted,
		&inst.ProductName, &inst.ProductVendor, &versionLabel,
	); err != nil {
		return nil, err
	}
	if vid.Valid {
		v := vid.Int64
		inst.VersionID = &v
	}
	if installed.Valid {
		t := installed.Time
		inst.InstalledAt = &t
	}
	if deleted.Valid {
		t := deleted.Time
		inst.DeletedAt = &t
	}
	if createdBy.Valid {
		v := createdBy.Int64
		inst.CreatedBy = &v
	}
	if updatedBy.Valid {
		v := updatedBy.Int64
		inst.UpdatedBy = &v
	}
	if versionLabel.Valid && strings.TrimSpace(versionLabel.String) != "" {
		inst.VersionLabel = versionLabel.String
	}
	return &inst, nil
}
