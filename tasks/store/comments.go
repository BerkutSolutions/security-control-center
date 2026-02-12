package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) AddTaskComment(ctx context.Context, comment *tasks.Comment) (int64, error) {
	now := time.Now().UTC()
	attachments := "[]"
	if comment.Attachments != nil {
		if raw, err := json.Marshal(comment.Attachments); err == nil {
			attachments = string(raw)
		}
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_comments(task_id, author_id, content, attachments, created_at) VALUES(?,?,?,?,?)`,
		comment.TaskID, comment.AuthorID, comment.Content, attachments, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	comment.ID = id
	comment.CreatedAt = now
	return id, nil
}

func (s *SQLiteStore) ListTaskComments(ctx context.Context, taskID int64) ([]tasks.Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, author_id, content, attachments, created_at
		FROM task_comments WHERE task_id=? ORDER BY created_at ASC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.Comment
	for rows.Next() {
		var c tasks.Comment
		var attachments string
		if err := rows.Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.Content, &attachments, &c.CreatedAt); err != nil {
			return nil, err
		}
		if attachments != "" {
			_ = json.Unmarshal([]byte(attachments), &c.Attachments)
		}
		res = append(res, c)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) GetTaskComment(ctx context.Context, commentID int64) (*tasks.Comment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, author_id, content, attachments, created_at
		FROM task_comments WHERE id=?`, commentID)
	var c tasks.Comment
	var attachments string
	if err := row.Scan(&c.ID, &c.TaskID, &c.AuthorID, &c.Content, &attachments, &c.CreatedAt); err != nil {
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

func (s *SQLiteStore) UpdateTaskComment(ctx context.Context, comment *tasks.Comment) error {
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
		UPDATE task_comments SET content=?, attachments=? WHERE id=?`,
		comment.Content, attachments, comment.ID)
	return err
}

func (s *SQLiteStore) DeleteTaskComment(ctx context.Context, commentID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM task_comments WHERE id=?`, commentID)
	return err
}
