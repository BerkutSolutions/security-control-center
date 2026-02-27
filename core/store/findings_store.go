package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Finding struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	DescriptionMD string     `json:"description_md"`
	Status        string     `json:"status"`
	Severity      string     `json:"severity"`
	FindingType   string     `json:"finding_type"`
	Owner         string     `json:"owner"`
	DueAt         *time.Time `json:"due_at,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	CreatedBy     *int64     `json:"created_by,omitempty"`
	UpdatedBy     *int64     `json:"updated_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Version       int        `json:"version"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

type FindingFilter struct {
	Search         string
	Status         string
	Severity       string
	Type           string
	Tag            string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

type FindingsStore interface {
	ListFindings(ctx context.Context, filter FindingFilter) ([]Finding, error)
	ListFindingsLite(ctx context.Context, search string, limit int) ([]Finding, error)
	SuggestFindingTitles(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestFindingOwners(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	SuggestFindingTags(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error)
	GetFinding(ctx context.Context, id int64) (*Finding, error)
	CreateFinding(ctx context.Context, f *Finding) (int64, error)
	UpdateFinding(ctx context.Context, f *Finding) error
	ArchiveFinding(ctx context.Context, id int64, updatedBy int64) error
	RestoreFinding(ctx context.Context, id int64, updatedBy int64) error
}

type findingsStore struct {
	db *sql.DB
}

func NewFindingsStore(db *sql.DB) FindingsStore {
	return &findingsStore{db: db}
}

func (s *findingsStore) ListFindings(ctx context.Context, filter FindingFilter) ([]Finding, error) {
	clauses := []string{}
	args := []any{}
	if !filter.IncludeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if q := strings.TrimSpace(filter.Search); q != "" {
		clauses = append(clauses, "(LOWER(title) LIKE ? OR LOWER(description_md) LIKE ? OR LOWER(owner) LIKE ?)")
		pattern := "%" + strings.ToLower(q) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Status)); v != "" {
		clauses = append(clauses, "LOWER(status)=?")
		args = append(args, v)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Severity)); v != "" {
		clauses = append(clauses, "LOWER(severity)=?")
		args = append(args, v)
	}
	if v := strings.ToLower(strings.TrimSpace(filter.Type)); v != "" {
		clauses = append(clauses, "LOWER(finding_type)=?")
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
		SELECT id, title, description_md, status, severity, finding_type, owner, due_at, tags_json,
		       created_by, updated_by, created_at, updated_at, version, deleted_at
		FROM findings`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY deleted_at IS NOT NULL, updated_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		var tagsRaw string
		var due, deleted sql.NullTime
		var createdBy, updatedBy sql.NullInt64
		if err := rows.Scan(&f.ID, &f.Title, &f.DescriptionMD, &f.Status, &f.Severity, &f.FindingType, &f.Owner, &due, &tagsRaw, &createdBy, &updatedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version, &deleted); err != nil {
			return nil, err
		}
		if due.Valid {
			t := due.Time
			f.DueAt = &t
		}
		if deleted.Valid {
			t := deleted.Time
			f.DeletedAt = &t
		}
		if createdBy.Valid {
			v := createdBy.Int64
			f.CreatedBy = &v
		}
		if updatedBy.Valid {
			v := updatedBy.Int64
			f.UpdatedBy = &v
		}
		if tagsRaw != "" {
			_ = json.Unmarshal([]byte(tagsRaw), &f.Tags)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *findingsStore) ListFindingsLite(ctx context.Context, search string, limit int) ([]Finding, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	args := []any{}
	clauses := []string{"deleted_at IS NULL"}
	if q := strings.TrimSpace(search); q != "" {
		clauses = append(clauses, "LOWER(title) LIKE ?")
		args = append(args, "%"+strings.ToLower(q)+"%")
	}
	query := `
		SELECT id, title, status, severity, finding_type, updated_at, version
		FROM findings
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.Title, &f.Status, &f.Severity, &f.FindingType, &f.UpdatedAt, &f.Version); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *findingsStore) GetFinding(ctx context.Context, id int64) (*Finding, error) {
	if id <= 0 {
		return nil, errors.New("bad id")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, description_md, status, severity, finding_type, owner, due_at, tags_json,
		       created_by, updated_by, created_at, updated_at, version, deleted_at
		FROM findings
		WHERE id=?`, id)
	var f Finding
	var tagsRaw string
	var due, deleted sql.NullTime
	var createdBy, updatedBy sql.NullInt64
	if err := row.Scan(&f.ID, &f.Title, &f.DescriptionMD, &f.Status, &f.Severity, &f.FindingType, &f.Owner, &due, &tagsRaw, &createdBy, &updatedBy, &f.CreatedAt, &f.UpdatedAt, &f.Version, &deleted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if due.Valid {
		t := due.Time
		f.DueAt = &t
	}
	if deleted.Valid {
		t := deleted.Time
		f.DeletedAt = &t
	}
	if createdBy.Valid {
		v := createdBy.Int64
		f.CreatedBy = &v
	}
	if updatedBy.Valid {
		v := updatedBy.Int64
		f.UpdatedBy = &v
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &f.Tags)
	}
	return &f, nil
}

func (s *findingsStore) CreateFinding(ctx context.Context, f *Finding) (int64, error) {
	if f == nil {
		return 0, errors.New("nil finding")
	}
	now := time.Now().UTC()
	tagsJSON, _ := json.Marshal(normalizeUpperTags(f.Tags))
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO findings(title, description_md, status, severity, finding_type, owner, due_at, tags_json, created_by, updated_by, created_at, updated_at, version)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,1)
	`, strings.TrimSpace(f.Title), strings.TrimSpace(f.DescriptionMD), normalizeFindingStatus(f.Status), normalizeFindingSeverity(f.Severity), normalizeFindingType(f.FindingType),
		strings.TrimSpace(f.Owner), f.DueAt, string(tagsJSON), nullableID(f.CreatedBy), nullableID(f.UpdatedBy), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (s *findingsStore) UpdateFinding(ctx context.Context, f *Finding) error {
	if f == nil || f.ID <= 0 {
		return errors.New("bad id")
	}
	now := time.Now().UTC()
	tagsJSON, _ := json.Marshal(normalizeUpperTags(f.Tags))
	res, err := s.db.ExecContext(ctx, `
		UPDATE findings
		SET title=?, description_md=?, status=?, severity=?, finding_type=?, owner=?, due_at=?, tags_json=?, updated_by=?, updated_at=?, version=version+1
		WHERE id=? AND version=? AND deleted_at IS NULL
	`, strings.TrimSpace(f.Title), strings.TrimSpace(f.DescriptionMD), normalizeFindingStatus(f.Status), normalizeFindingSeverity(f.Severity), normalizeFindingType(f.FindingType),
		strings.TrimSpace(f.Owner), f.DueAt, string(tagsJSON), nullableID(f.UpdatedBy), now, f.ID, f.Version)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrConflict
	}
	return nil
}

func (s *findingsStore) ArchiveFinding(ctx context.Context, id int64, updatedBy int64) error {
	if id <= 0 {
		return errors.New("bad id")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE findings SET deleted_at=?, updated_by=?, updated_at=?, version=version+1
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

func (s *findingsStore) RestoreFinding(ctx context.Context, id int64, updatedBy int64) error {
	if id <= 0 {
		return errors.New("bad id")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE findings SET deleted_at=NULL, updated_by=?, updated_at=?, version=version+1
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

func normalizeFindingStatus(v string) string {
	val := strings.ToLower(strings.TrimSpace(v))
	switch val {
	case "open", "in_progress", "resolved", "accepted_risk", "false_positive":
		return val
	default:
		return "open"
	}
}

func normalizeFindingSeverity(v string) string {
	val := strings.ToLower(strings.TrimSpace(v))
	switch val {
	case "critical", "high", "medium", "low":
		return val
	default:
		return "medium"
	}
}

func normalizeFindingType(v string) string {
	val := strings.ToLower(strings.TrimSpace(v))
	switch val {
	case "config", "process", "technical", "compliance", "other":
		return val
	default:
		return "other"
	}
}

func normalizeUpperTags(tags []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, t := range tags {
		val := strings.ToUpper(strings.TrimSpace(t))
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}
