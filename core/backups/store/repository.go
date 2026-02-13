package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"berkut-scc/core/backups"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListArtifacts(ctx context.Context, filter backups.ListArtifactsFilter) ([]backups.BackupArtifact, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, source, created_by_user_id, origin_filename, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at
		FROM backups_artifacts
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]backups.BackupArtifact, 0)
	for rows.Next() {
		item, err := scanBackupArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetArtifact(ctx context.Context, id int64) (*backups.BackupArtifact, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, run_id, source, created_by_user_id, origin_filename, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at
		FROM backups_artifacts
		WHERE id=?
	`, id)
	item, err := scanBackupArtifact(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, backups.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetArtifactByStoragePath(ctx context.Context, storagePath string) (*backups.BackupArtifact, error) {
	path := strings.TrimSpace(storagePath)
	if path == "" {
		return nil, backups.ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, run_id, source, created_by_user_id, origin_filename, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at
		FROM backups_artifacts
		WHERE storage_path=?
		ORDER BY id DESC
		LIMIT 1
	`, path)
	item, err := scanBackupArtifact(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, backups.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateRun(ctx context.Context, run *backups.BackupRun) (*backups.BackupRun, error) {
	if run == nil {
		return nil, sql.ErrNoRows
	}
	now := time.Now().UTC()
	if run.MetaJSON == nil {
		run.MetaJSON = json.RawMessage("{}")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO backups_runs(artifact_id, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)
	`, run.ArtifactID, run.Status, run.SizeBytes, run.Checksum, run.Filename, run.StoragePath, run.ErrorCode, run.ErrorMessage, run.MetaJSON, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	run.ID = id
	run.CreatedAt = now
	run.UpdatedAt = now
	return run, nil
}

func (r *Repository) UpdateRun(ctx context.Context, run *backups.BackupRun) error {
	if run == nil {
		return sql.ErrNoRows
	}
	run.UpdatedAt = time.Now().UTC()
	if run.MetaJSON == nil {
		run.MetaJSON = json.RawMessage("{}")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE backups_runs
		SET artifact_id=?, status=?, size_bytes=?, checksum=?, filename=?, storage_path=?, error_code=?, error_message=?, meta_json=?, updated_at=?
		WHERE id=?
	`, run.ArtifactID, run.Status, run.SizeBytes, run.Checksum, run.Filename, run.StoragePath, run.ErrorCode, run.ErrorMessage, run.MetaJSON, run.UpdatedAt, run.ID)
	return err
}

func (r *Repository) CreateArtifact(ctx context.Context, artifact *backups.BackupArtifact) (*backups.BackupArtifact, error) {
	if artifact == nil {
		return nil, sql.ErrNoRows
	}
	now := time.Now().UTC()
	if artifact.MetaJSON == nil {
		artifact.MetaJSON = json.RawMessage("{}")
	}
	if artifact.Source == "" {
		artifact.Source = backups.ArtifactSourceLocal
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO backups_artifacts(run_id, source, created_by_user_id, origin_filename, status, size_bytes, checksum, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, artifact.RunID, artifact.Source, artifact.CreatedByID, artifact.OriginFile, artifact.Status, artifact.SizeBytes, artifact.Checksum, artifact.Filename, artifact.StoragePath, artifact.ErrorCode, artifact.ErrorMessage, artifact.MetaJSON, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	artifact.ID = id
	artifact.CreatedAt = now
	artifact.UpdatedAt = now
	return artifact, nil
}

func (r *Repository) AttachArtifactToRun(ctx context.Context, runID, artifactID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE backups_runs SET artifact_id=?, updated_at=? WHERE id=?`, artifactID, time.Now().UTC(), runID)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE backups_artifacts SET run_id=?, updated_at=? WHERE id=?`, runID, time.Now().UTC(), artifactID)
	return err
}

func (r *Repository) GetGooseVersion(ctx context.Context) (int64, error) {
	var version sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied=TRUE`).Scan(&version)
	if err != nil {
		return 0, err
	}
	if !version.Valid {
		return 0, nil
	}
	return version.Int64, nil
}

func (r *Repository) GetRestoreRun(ctx context.Context, id int64) (*backups.RestoreRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, artifact_id, status, size_bytes, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at
		FROM backups_restore_runs
		WHERE id=?
	`, id)
	item, err := scanRestoreRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, backups.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *Repository) CreateRestoreRun(ctx context.Context, run *backups.RestoreRun) (*backups.RestoreRun, error) {
	if run == nil {
		return nil, sql.ErrNoRows
	}
	now := time.Now().UTC()
	if run.MetaJSON == nil {
		run.MetaJSON = json.RawMessage("{}")
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO backups_restore_runs(artifact_id, status, size_bytes, filename, storage_path, error_code, error_message, meta_json, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)
	`, run.ArtifactID, run.Status, run.SizeBytes, run.Filename, run.StoragePath, run.ErrorCode, run.ErrorMessage, run.MetaJSON, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	run.ID = id
	run.CreatedAt = now
	run.UpdatedAt = now
	return run, nil
}

func (r *Repository) UpdateRestoreRun(ctx context.Context, run *backups.RestoreRun) error {
	if run == nil {
		return sql.ErrNoRows
	}
	run.UpdatedAt = time.Now().UTC()
	if run.MetaJSON == nil {
		run.MetaJSON = json.RawMessage("{}")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE backups_restore_runs
		SET status=?, size_bytes=?, filename=?, storage_path=?, error_code=?, error_message=?, meta_json=?, updated_at=?
		WHERE id=?
	`, run.Status, run.SizeBytes, run.Filename, run.StoragePath, run.ErrorCode, run.ErrorMessage, run.MetaJSON, run.UpdatedAt, run.ID)
	return err
}

func (r *Repository) GetMaintenanceMode(ctx context.Context) (bool, error) {
	var enabled bool
	err := r.db.QueryRowContext(ctx, `SELECT enabled FROM app_maintenance_state WHERE id=1`).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return enabled, nil
}

func (r *Repository) SetMaintenanceMode(ctx context.Context, enabled bool, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO app_maintenance_state(id, enabled, reason, updated_at)
		VALUES(1,?,?,?)
		ON CONFLICT (id) DO UPDATE SET enabled=EXCLUDED.enabled, reason=EXCLUDED.reason, updated_at=EXCLUDED.updated_at
	`, enabled, reason, time.Now().UTC())
	return err
}

func (r *Repository) ResetRunningOperations(ctx context.Context) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE backups_runs
		SET status='failed',
			error_code='backups.restore.recovered',
			error_message='state reset after restore',
			updated_at=?
		WHERE status IN ('queued','running')
	`, now)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE backups_restore_runs
		SET status='failed',
			error_code='backups.restore.recovered',
			error_message='state reset after restore',
			updated_at=?
		WHERE status IN ('queued','running')
	`, now)
	return err
}

type artifactScanner interface {
	Scan(dest ...any) error
}

func scanBackupArtifact(s artifactScanner) (backups.BackupArtifact, error) {
	item := backups.BackupArtifact{}
	var runID sql.NullInt64
	var source sql.NullString
	var createdByID sql.NullInt64
	var originFilename sql.NullString
	var size sql.NullInt64
	var checksum sql.NullString
	var filename sql.NullString
	var storagePath sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var meta []byte
	err := s.Scan(
		&item.ID,
		&runID,
		&source,
		&createdByID,
		&originFilename,
		&item.Status,
		&size,
		&checksum,
		&filename,
		&storagePath,
		&errorCode,
		&errorMessage,
		&meta,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return backups.BackupArtifact{}, err
	}
	item.SizeBytes = nullableInt64(size)
	item.RunID = nullableInt64(runID)
	if source.Valid {
		item.Source = source.String
	} else {
		item.Source = backups.ArtifactSourceLocal
	}
	item.CreatedByID = nullableInt64(createdByID)
	item.OriginFile = nullableString(originFilename)
	item.Checksum = nullableString(checksum)
	item.Filename = nullableString(filename)
	item.StoragePath = nullableString(storagePath)
	item.ErrorCode = nullableString(errorCode)
	item.ErrorMessage = nullableString(errorMessage)
	item.MetaJSON = safeMeta(meta)
	return item, nil
}

func scanRestoreRun(s artifactScanner) (backups.RestoreRun, error) {
	item := backups.RestoreRun{}
	var size sql.NullInt64
	var filename sql.NullString
	var storagePath sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var meta []byte
	err := s.Scan(
		&item.ID,
		&item.ArtifactID,
		&item.Status,
		&size,
		&filename,
		&storagePath,
		&errorCode,
		&errorMessage,
		&meta,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return backups.RestoreRun{}, err
	}
	item.SizeBytes = nullableInt64(size)
	item.Filename = nullableString(filename)
	item.StoragePath = nullableString(storagePath)
	item.ErrorCode = nullableString(errorCode)
	item.ErrorMessage = nullableString(errorMessage)
	item.MetaJSON = safeMeta(meta)
	item.DryRun, item.Steps = decodeRestoreMeta(item.MetaJSON)
	return item, nil
}

func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	val := v.Int64
	return &val
}

func nullableString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	val := v.String
	return &val
}

func safeMeta(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(raw)
}

func decodeRestoreMeta(raw json.RawMessage) (bool, []backups.RestoreStep) {
	type restoreMeta struct {
		Mode  string                `json:"mode"`
		Steps []backups.RestoreStep `json:"steps"`
	}
	meta := restoreMeta{}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return false, nil
	}
	return meta.Mode == "dry_run", meta.Steps
}
