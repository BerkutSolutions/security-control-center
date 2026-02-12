package store

import (
	"context"
	"strings"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) ListEntityLinks(ctx context.Context, sourceType, sourceID string) ([]tasks.Link, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source_type, source_id, target_type, target_id, created_at
		FROM entity_links WHERE source_type=? AND source_id=? ORDER BY created_at DESC, id DESC`,
		strings.ToLower(strings.TrimSpace(sourceType)), strings.TrimSpace(sourceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Link
	for rows.Next() {
		var l tasks.Link
		if err := rows.Scan(&l.ID, &l.SourceType, &l.SourceID, &l.TargetType, &l.TargetID, &l.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) AddEntityLink(ctx context.Context, link *tasks.Link) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO entity_links(source_type, source_id, target_type, target_id, created_at)
		VALUES(?,?,?,?,?)`,
		strings.ToLower(strings.TrimSpace(link.SourceType)),
		strings.TrimSpace(link.SourceID),
		strings.ToLower(strings.TrimSpace(link.TargetType)),
		strings.TrimSpace(link.TargetID),
		now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	link.ID = id
	link.CreatedAt = now
	return id, nil
}

func (s *SQLiteStore) DeleteEntityLink(ctx context.Context, linkID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM entity_links WHERE id=?`, linkID)
	return err
}
