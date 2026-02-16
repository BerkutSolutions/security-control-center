package store

import (
	"context"
	"database/sql"

	"berkut-scc/core/backups"
)

func (r *Repository) HasRunningBackupRun(ctx context.Context) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM backups_runs WHERE status IN ('queued', 'running')`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) HasRunningRestoreRun(ctx context.Context) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM backups_restore_runs WHERE status IN ('queued', 'running')`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ListSuccessfulArtifacts(ctx context.Context, limit int) ([]backups.BackupArtifact, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, source, created_by_user_id, origin_filename, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at
		FROM backups_artifacts
		WHERE status='success'
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]backups.BackupArtifact, 0)
	for rows.Next() {
		item, scanErr := scanBackupArtifact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteArtifact(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM backups_artifacts WHERE id=?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
