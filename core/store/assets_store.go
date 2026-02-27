package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

type Asset struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	Description    string     `json:"description"`
	CommissionedAt *time.Time `json:"commissioned_at,omitempty"`
	IPAddresses    []string   `json:"ip_addresses,omitempty"`
	Criticality    string     `json:"criticality"`
	Owner          string     `json:"owner"`
	Administrator  string     `json:"administrator"`
	Env            string     `json:"env"`
	Status         string     `json:"status"`
	Tags           []string   `json:"tags,omitempty"`
	CreatedBy      *int64     `json:"created_by,omitempty"`
	UpdatedBy      *int64     `json:"updated_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	Version        int        `json:"version"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type AssetLite struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type AssetFilter struct {
	Search        string
	Type          string
	Criticality   string
	Env           string
	Status        string
	Tag           string
	IncludeDeleted bool
	Limit         int
	Offset        int
}

type AssetsStore interface {
	ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, error)
	ListAssetsLite(ctx context.Context, search string, limit int) ([]AssetLite, error)
	SuggestAssetOwners(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestAssetAdministrators(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestAssetTags(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	GetAsset(ctx context.Context, id int64) (*Asset, error)
	CreateAsset(ctx context.Context, a *Asset) (int64, error)
	UpdateAsset(ctx context.Context, a *Asset) error
	ArchiveAsset(ctx context.Context, id int64, updatedBy int64) error
	RestoreAsset(ctx context.Context, id int64, updatedBy int64) error
}

type assetsStore struct {
	db *sql.DB
}

func NewAssetsStore(db *sql.DB) AssetsStore {
	return &assetsStore{db: db}
}

func (s *assetsStore) ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, error) {
	clauses := []string{}
	args := []any{}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		clauses = append(clauses, "(LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(owner) LIKE ? OR LOWER(administrator) LIKE ? OR ip_addresses_json LIKE ?)")
		pattern := "%" + strings.ToLower(q) + "%"
		args = append(args, pattern, pattern, pattern, pattern, "%"+q+"%")
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Type)); v != "" {
		clauses = append(clauses, "LOWER(type)=?")
		args = append(args, v)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Criticality)); v != "" {
		clauses = append(clauses, "LOWER(criticality)=?")
		args = append(args, v)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Env)); v != "" {
		clauses = append(clauses, "LOWER(env)=?")
		args = append(args, v)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Status)); v != "" {
		clauses = append(clauses, "LOWER(status)=?")
		args = append(args, v)
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

	query := `
		SELECT id, name, type, description, commissioned_at, ip_addresses_json, criticality, owner, administrator, env, status, tags_json,
		       created_by, updated_by, created_at, updated_at, version, deleted_at
		FROM assets`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY LOWER(name) ASC, id ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Asset
	for rows.Next() {
		item, err := scanAssetRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (s *assetsStore) ListAssetsLite(ctx context.Context, search string, limit int) ([]AssetLite, error) {
	clauses := []string{"deleted_at IS NULL"}
	args := []any{}
	if q := strings.TrimSpace(search); q != "" {
		clauses = append(clauses, "(LOWER(name) LIKE ? OR LOWER(type) LIKE ?)")
		pattern := "%" + strings.ToLower(q) + "%"
		args = append(args, pattern, pattern)
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
		SELECT id, name, type
		FROM assets
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY LOWER(name) ASC, id ASC
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetLite
	for rows.Next() {
		var a AssetLite
		if err := rows.Scan(&a.ID, &a.Name, &a.Type); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *assetsStore) GetAsset(ctx context.Context, id int64) (*Asset, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, type, description, commissioned_at, ip_addresses_json, criticality, owner, administrator, env, status, tags_json,
		       created_by, updated_by, created_at, updated_at, version, deleted_at
		FROM assets
		WHERE id=?`, id)
	return scanAsset(row)
}

func (s *assetsStore) CreateAsset(ctx context.Context, a *Asset) (int64, error) {
	if a == nil {
		return 0, errors.New("nil asset")
	}
	now := time.Now().UTC()
	a.Name = strings.TrimSpace(a.Name)
	a.Description = strings.TrimSpace(a.Description)
	a.Owner = strings.TrimSpace(a.Owner)
	a.Administrator = strings.TrimSpace(a.Administrator)
	a.IPAddresses = normalizeIPs(a.IPAddresses)
	a.Tags = normalizeTags(a.Tags)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO assets(name, type, description, commissioned_at, ip_addresses_json, criticality, owner, administrator, env, status, tags_json, created_by, updated_by, created_at, updated_at, version)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.Name, a.Type, a.Description, nullableTime(a.CommissionedAt), ipsToJSON(a.IPAddresses), a.Criticality, a.Owner, a.Administrator, a.Env, a.Status, tagsToJSON(a.Tags), nullableID(a.CreatedBy), nullableID(a.UpdatedBy), now, now, 1)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	a.CreatedAt = now
	a.UpdatedAt = now
	a.Version = 1
	return id, nil
}

func (s *assetsStore) UpdateAsset(ctx context.Context, a *Asset) error {
	if a == nil || a.ID <= 0 {
		return errors.New("invalid asset")
	}
	now := time.Now().UTC()
	a.Name = strings.TrimSpace(a.Name)
	a.Description = strings.TrimSpace(a.Description)
	a.Owner = strings.TrimSpace(a.Owner)
	a.Administrator = strings.TrimSpace(a.Administrator)
	a.IPAddresses = normalizeIPs(a.IPAddresses)
	a.Tags = normalizeTags(a.Tags)

	res, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET name=?, type=?, description=?, commissioned_at=?, ip_addresses_json=?, criticality=?, owner=?, administrator=?, env=?, status=?, tags_json=?,
		    updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND deleted_at IS NULL`,
		a.Name, a.Type, a.Description, nullableTime(a.CommissionedAt), ipsToJSON(a.IPAddresses), a.Criticality, a.Owner, a.Administrator, a.Env, a.Status, tagsToJSON(a.Tags),
		nullableID(a.UpdatedBy), now, a.ID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	a.UpdatedAt = now
	return nil
}

func (s *assetsStore) ArchiveAsset(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET deleted_at=?, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND deleted_at IS NULL`, now, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *assetsStore) RestoreAsset(ctx context.Context, id int64, updatedBy int64) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET deleted_at=NULL, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND deleted_at IS NOT NULL`, updatedBy, now, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanAsset(row *sql.Row) (*Asset, error) {
	var a Asset
	var commissioned sql.NullTime
	var ipJSON string
	var tagsJSON string
	var createdBy sql.NullInt64
	var updatedBy sql.NullInt64
	if err := row.Scan(
		&a.ID, &a.Name, &a.Type, &a.Description, &commissioned, &ipJSON, &a.Criticality, &a.Owner, &a.Administrator, &a.Env, &a.Status, &tagsJSON,
		&createdBy, &updatedBy, &a.CreatedAt, &a.UpdatedAt, &a.Version, &a.DeletedAt,
	); err != nil {
		return nil, err
	}
	if commissioned.Valid {
		a.CommissionedAt = &commissioned.Time
	}
	if createdBy.Valid {
		v := createdBy.Int64
		a.CreatedBy = &v
	}
	if updatedBy.Valid {
		v := updatedBy.Int64
		a.UpdatedBy = &v
	}
	_ = json.Unmarshal([]byte(ipJSON), &a.IPAddresses)
	_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
	return &a, nil
}

type assetRowScanner interface {
	Scan(dest ...any) error
}

func scanAssetRow(rows assetRowScanner) (*Asset, error) {
	var a Asset
	var commissioned sql.NullTime
	var ipJSON string
	var tagsJSON string
	var createdBy sql.NullInt64
	var updatedBy sql.NullInt64
	if err := rows.Scan(
		&a.ID, &a.Name, &a.Type, &a.Description, &commissioned, &ipJSON, &a.Criticality, &a.Owner, &a.Administrator, &a.Env, &a.Status, &tagsJSON,
		&createdBy, &updatedBy, &a.CreatedAt, &a.UpdatedAt, &a.Version, &a.DeletedAt,
	); err != nil {
		return nil, err
	}
	if commissioned.Valid {
		a.CommissionedAt = &commissioned.Time
	}
	if createdBy.Valid {
		v := createdBy.Int64
		a.CreatedBy = &v
	}
	if updatedBy.Valid {
		v := updatedBy.Int64
		a.UpdatedBy = &v
	}
	_ = json.Unmarshal([]byte(ipJSON), &a.IPAddresses)
	_ = json.Unmarshal([]byte(tagsJSON), &a.Tags)
	return &a, nil
}

func ipsToJSON(ips []string) string {
	if ips == nil {
		ips = []string{}
	}
	b, _ := json.Marshal(ips)
	return string(b)
}

func normalizeIPs(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		ip := net.ParseIP(v)
		if ip == nil {
			continue
		}
		canon := ip.String()
		if _, ok := seen[canon]; ok {
			continue
		}
		seen[canon] = struct{}{}
		out = append(out, canon)
	}
	return out
}

func validateIPs(in []string) error {
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if net.ParseIP(v) == nil {
			return fmt.Errorf("invalid ip: %s", v)
		}
	}
	return nil
}
