package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) ListTaskFiles(ctx context.Context, taskID int64) ([]tasks.TaskFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, name, stored_name, content_type, size_bytes, uploaded_by, uploaded_at
		FROM task_files WHERE task_id=? ORDER BY uploaded_at DESC, id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []tasks.TaskFile
	for rows.Next() {
		var f tasks.TaskFile
		var uploadedBy sql.NullInt64
		if err := rows.Scan(&f.ID, &f.TaskID, &f.Name, &f.StoredName, &f.ContentType, &f.Size, &uploadedBy, &f.UploadedAt); err != nil {
			return nil, err
		}
		if uploadedBy.Valid {
			f.UploadedBy = &uploadedBy.Int64
		}
		res = append(res, f)
	}
	return res, rows.Err()
}

func (s *SQLiteStore) AddTaskFile(ctx context.Context, file *tasks.TaskFile) (int64, error) {
	if file == nil {
		return 0, errors.New("missing file")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO task_files(task_id, name, stored_name, content_type, size_bytes, uploaded_by, uploaded_at)
		VALUES(?,?,?,?,?,?,?)`,
		file.TaskID, file.Name, file.StoredName, file.ContentType, file.Size, nullableID(file.UploadedBy), now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	file.ID = id
	file.UploadedAt = now
	return id, nil
}

func (s *SQLiteStore) GetTaskFile(ctx context.Context, taskID, fileID int64) (*tasks.TaskFile, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, name, stored_name, content_type, size_bytes, uploaded_by, uploaded_at
		FROM task_files WHERE id=? AND task_id=?`, fileID, taskID)
	var f tasks.TaskFile
	var uploadedBy sql.NullInt64
	if err := row.Scan(&f.ID, &f.TaskID, &f.Name, &f.StoredName, &f.ContentType, &f.Size, &uploadedBy, &f.UploadedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if uploadedBy.Valid {
		f.UploadedBy = &uploadedBy.Int64
	}
	return &f, nil
}

func (s *SQLiteStore) DeleteTaskFile(ctx context.Context, taskID, fileID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM task_files WHERE id=? AND task_id=?`, fileID, taskID)
	return err
}
