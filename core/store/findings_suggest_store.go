package store

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *findingsStore) SuggestFindingTitles(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error) {
	return s.suggestDistinctColumn(ctx, "title", search, limit, includeDeleted)
}

func (s *findingsStore) SuggestFindingOwners(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error) {
	return s.suggestDistinctColumn(ctx, "owner", search, limit, includeDeleted)
}

func (s *findingsStore) SuggestFindingTags(ctx context.Context, search string, limit int, includeDeleted bool) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	q := strings.ToUpper(strings.TrimSpace(search))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	clauses := []string{}
	args := []any{}
	if !includeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if q != "" {
		clauses = append(clauses, "tags_json LIKE ?")
		args = append(args, "%"+q+"%")
	}

	query := `SELECT tags_json FROM findings`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT 1000"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var tags []string
		_ = json.Unmarshal([]byte(raw), &tags)
		for _, t := range tags {
			tag := strings.ToUpper(strings.TrimSpace(t))
			if tag == "" {
				continue
			}
			if q != "" && !strings.Contains(tag, q) {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, rows.Err()
}

func (s *findingsStore) suggestDistinctColumn(ctx context.Context, column string, search string, limit int, includeDeleted bool) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	col := strings.ToLower(strings.TrimSpace(column))
	if col != "title" && col != "owner" {
		return nil, nil
	}
	q := strings.TrimSpace(search)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	clauses := []string{col + " <> ''"}
	args := []any{}
	if !includeDeleted {
		clauses = append(clauses, "deleted_at IS NULL")
	}
	if q != "" {
		clauses = append(clauses, "LOWER("+col+") LIKE ?")
		args = append(args, "%"+strings.ToLower(q)+"%")
	}
	query := "SELECT DISTINCT " + col + " FROM findings WHERE " + strings.Join(clauses, " AND ") + " ORDER BY LOWER(" + col + ") ASC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
