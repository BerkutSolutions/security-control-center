package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type ControlCommentAttachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
}

type ControlComment struct {
	ID          int64                     `json:"id"`
	ControlID   int64                     `json:"control_id"`
	AuthorID    int64                     `json:"author_id"`
	Content     string                    `json:"content"`
	Attachments []ControlCommentAttachment `json:"attachments"`
	CreatedAt   time.Time                 `json:"created_at"`
}

func (s *controlsStore) AddControlComment(ctx context.Context, comment *ControlComment) (int64, error) {
	now := time.Now().UTC()
	attachments := "[]"
	if comment.Attachments != nil {
		if raw, err := json.Marshal(comment.Attachments); err == nil {
			attachments = string(raw)
		}
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO control_comments(control_id, author_id, content, attachments, created_at)
		VALUES(?,?,?,?,?)`, comment.ControlID, comment.AuthorID, comment.Content, attachments, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	comment.ID = id
	comment.CreatedAt = now
	return id, nil
}

func (s *controlsStore) ListControlComments(ctx context.Context, controlID int64) ([]ControlComment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, control_id, author_id, content, attachments, created_at
		FROM control_comments WHERE control_id=? ORDER BY created_at ASC`, controlID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ControlComment
	for rows.Next() {
		var c ControlComment
		var attachments string
		if err := rows.Scan(&c.ID, &c.ControlID, &c.AuthorID, &c.Content, &attachments, &c.CreatedAt); err != nil {
			return nil, err
		}
		if attachments != "" {
			_ = json.Unmarshal([]byte(attachments), &c.Attachments)
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

func (s *controlsStore) GetControlComment(ctx context.Context, commentID int64) (*ControlComment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, control_id, author_id, content, attachments, created_at
		FROM control_comments WHERE id=?`, commentID)
	var c ControlComment
	var attachments string
	if err := row.Scan(&c.ID, &c.ControlID, &c.AuthorID, &c.Content, &attachments, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if attachments != "" {
		_ = json.Unmarshal([]byte(attachments), &c.Attachments)
	}
	return &c, nil
}

func (s *controlsStore) UpdateControlComment(ctx context.Context, comment *ControlComment) error {
	if comment == nil {
		return nil
	}
	attachments := "[]"
	if comment.Attachments != nil {
		if raw, err := json.Marshal(comment.Attachments); err == nil {
			attachments = string(raw)
		}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE control_comments SET content=?, attachments=? WHERE id=?`, comment.Content, attachments, comment.ID)
	return err
}

func (s *controlsStore) DeleteControlComment(ctx context.Context, commentID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM control_comments WHERE id=?`, commentID)
	return err
}
