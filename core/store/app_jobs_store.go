package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type AppJob struct {
	ID int64 `json:"id"`

	Type     string `json:"type"`
	Scope    string `json:"scope"`
	ModuleID string `json:"module_id,omitempty"`
	Mode     string `json:"mode"`

	Status   string `json:"status"`
	Progress int    `json:"progress"`

	StartedBy string     `json:"started_by"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	LogJSON string `json:"log_json"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AppJobCreate struct {
	Type     string
	Scope    string // "module" | "all"
	ModuleID string
	Mode     string // "full" | "partial"
	StartedBy string
}

type AppJobsStore interface {
	Create(ctx context.Context, in AppJobCreate) (int64, error)
	Get(ctx context.Context, id int64) (*AppJob, error)
	ListRecent(ctx context.Context, limit int) ([]AppJob, error)

	NextQueued(ctx context.Context) (*AppJob, error)
	MarkRunning(ctx context.Context, id int64, startedAt time.Time) (bool, error)
	UpdateProgress(ctx context.Context, id int64, progress int, logJSON string) error
	Finish(ctx context.Context, id int64, status string, finishedAt time.Time, progress int, logJSON string) error
	Cancel(ctx context.Context, id int64, now time.Time) (bool, error)
}

type appJobsStore struct {
	db *sql.DB
}

func NewAppJobsStore(db *sql.DB) AppJobsStore {
	return &appJobsStore{db: db}
}

func (s *appJobsStore) Create(ctx context.Context, in AppJobCreate) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("db not configured")
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO app_jobs(
			type, scope, module_id, mode, status, progress,
			started_by, started_at, finished_at, log_json, created_at, updated_at
		)
		VALUES(?,?,?,?, 'queued', 0, ?, NULL, NULL, '[]', ?, ?)
	`, in.Type, in.Scope, in.ModuleID, in.Mode, in.StartedBy, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *appJobsStore) Get(ctx context.Context, id int64) (*AppJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("db not configured")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, scope, module_id, mode, status, progress,
			started_by, started_at, finished_at, log_json, created_at, updated_at
		FROM app_jobs
		WHERE id=?
	`, id)
	return scanAppJob(row)
}

func (s *appJobsStore) ListRecent(ctx context.Context, limit int) ([]AppJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("db not configured")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, scope, module_id, mode, status, progress,
			started_by, started_at, finished_at, log_json, created_at, updated_at
		FROM app_jobs
		ORDER BY created_at DESC, id DESC
		LIMIT `+intToString(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppJob
	for rows.Next() {
		item, err := scanAppJob(rows)
		if err != nil {
			return nil, err
		}
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, rows.Err()
}

func (s *appJobsStore) NextQueued(ctx context.Context) (*AppJob, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("db not configured")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, type, scope, module_id, mode, status, progress,
			started_by, started_at, finished_at, log_json, created_at, updated_at
		FROM app_jobs
		WHERE status='queued'
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`)
	job, err := scanAppJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return job, err
}

func (s *appJobsStore) MarkRunning(ctx context.Context, id int64, startedAt time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("db not configured")
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE app_jobs
		SET status='running', started_at=?, updated_at=?
		WHERE id=? AND status='queued'
	`, startedAt.UTC(), time.Now().UTC(), id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *appJobsStore) UpdateProgress(ctx context.Context, id int64, progress int, logJSON string) error {
	if s == nil || s.db == nil {
		return errors.New("db not configured")
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE app_jobs
		SET progress=?, log_json=?, updated_at=?
		WHERE id=?
	`, progress, logJSON, time.Now().UTC(), id)
	return err
}

func (s *appJobsStore) Finish(ctx context.Context, id int64, status string, finishedAt time.Time, progress int, logJSON string) error {
	if s == nil || s.db == nil {
		return errors.New("db not configured")
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE app_jobs
		SET status=?, progress=?, finished_at=?, log_json=?, updated_at=?
		WHERE id=?
	`, status, progress, finishedAt.UTC(), logJSON, time.Now().UTC(), id)
	return err
}

func (s *appJobsStore) Cancel(ctx context.Context, id int64, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("db not configured")
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE app_jobs
		SET status='canceled', progress=100, finished_at=?, updated_at=?
		WHERE id=? AND status IN ('queued','running')
	`, now.UTC(), time.Now().UTC(), id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

type scanRow interface {
	Scan(dest ...any) error
}

func scanAppJob(row scanRow) (*AppJob, error) {
	var job AppJob
	var startedAt sql.NullTime
	var finishedAt sql.NullTime
	var moduleID string
	if err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Scope,
		&moduleID,
		&job.Mode,
		&job.Status,
		&job.Progress,
		&job.StartedBy,
		&startedAt,
		&finishedAt,
		&job.LogJSON,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return nil, err
	}
	job.ModuleID = moduleID
	if startedAt.Valid {
		t := startedAt.Time.UTC()
		job.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		job.FinishedAt = &t
	}
	return &job, nil
}

