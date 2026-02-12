package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type EntityLink struct {
	ID           int64     `json:"id"`
	SourceType   string    `json:"source_type"`
	SourceID     string    `json:"source_id"`
	TargetType   string    `json:"target_type"`
	TargetID     string    `json:"target_id"`
	RelationType string    `json:"relation_type"`
	CreatedAt    time.Time `json:"created_at"`
}

type EntityLinksStore interface {
	ListBySource(ctx context.Context, sourceType, sourceID string) ([]EntityLink, error)
	ListByTarget(ctx context.Context, targetType, targetID string) ([]EntityLink, error)
	Add(ctx context.Context, link *EntityLink) (int64, error)
	Delete(ctx context.Context, linkID int64) error
}

type entityLinksStore struct {
	db *sql.DB
}

func NewEntityLinksStore(db *sql.DB) EntityLinksStore {
	return &entityLinksStore{db: db}
}

func (s *entityLinksStore) ListBySource(ctx context.Context, sourceType, sourceID string) ([]EntityLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source_type, source_id, target_type, target_id, relation_type, created_at
		FROM entity_links
		WHERE source_type=? AND source_id=?
		ORDER BY created_at DESC, id DESC`,
		strings.ToLower(strings.TrimSpace(sourceType)), strings.TrimSpace(sourceID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []EntityLink
	for rows.Next() {
		var l EntityLink
		if err := rows.Scan(&l.ID, &l.SourceType, &l.SourceID, &l.TargetType, &l.TargetID, &l.RelationType, &l.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, rows.Err()
}

func (s *entityLinksStore) ListByTarget(ctx context.Context, targetType, targetID string) ([]EntityLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source_type, source_id, target_type, target_id, relation_type, created_at
		FROM entity_links
		WHERE target_type=? AND target_id=?
		ORDER BY created_at DESC, id DESC`,
		strings.ToLower(strings.TrimSpace(targetType)), strings.TrimSpace(targetID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []EntityLink
	for rows.Next() {
		var l EntityLink
		if err := rows.Scan(&l.ID, &l.SourceType, &l.SourceID, &l.TargetType, &l.TargetID, &l.RelationType, &l.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, l)
	}
	return res, rows.Err()
}

func (s *entityLinksStore) Add(ctx context.Context, link *EntityLink) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO entity_links(source_type, source_id, target_type, target_id, relation_type, created_at)
		VALUES(?,?,?,?,?,?)`,
		strings.ToLower(strings.TrimSpace(link.SourceType)),
		strings.TrimSpace(link.SourceID),
		strings.ToLower(strings.TrimSpace(link.TargetType)),
		strings.TrimSpace(link.TargetID),
		strings.ToLower(strings.TrimSpace(link.RelationType)),
		now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	link.ID = id
	link.CreatedAt = now
	return id, nil
}

func (s *entityLinksStore) Delete(ctx context.Context, linkID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM entity_links WHERE id=?`, linkID)
	return err
}
