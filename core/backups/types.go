package backups

import (
	"encoding/json"
	"io"
	"time"
)

type RunStatus string

const (
	StatusQueued   RunStatus = "queued"
	StatusRunning  RunStatus = "running"
	StatusSuccess  RunStatus = "success"
	StatusFailed   RunStatus = "failed"
	StatusCanceled RunStatus = "canceled"
)

const (
	ArtifactSourceLocal    = "local"
	ArtifactSourceImported = "imported"
)

type BackupArtifact struct {
	ID           int64           `json:"id"`
	RunID        *int64          `json:"run_id,omitempty"`
	Source       string          `json:"source,omitempty"`
	CreatedByID  *int64          `json:"created_by_user_id,omitempty"`
	OriginFile   *string         `json:"origin_filename,omitempty"`
	Status       RunStatus       `json:"status"`
	SizeBytes    *int64          `json:"size_bytes,omitempty"`
	Checksum     *string         `json:"checksum,omitempty"`
	Filename     *string         `json:"filename,omitempty"`
	StoragePath  *string         `json:"storage_path,omitempty"`
	ErrorCode    *string         `json:"error_code,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	MetaJSON     json.RawMessage `json:"meta_json"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type BackupRun struct {
	ID           int64           `json:"id"`
	ArtifactID   *int64          `json:"artifact_id,omitempty"`
	Status       RunStatus       `json:"status"`
	SizeBytes    *int64          `json:"size_bytes,omitempty"`
	Checksum     *string         `json:"checksum,omitempty"`
	Filename     *string         `json:"filename,omitempty"`
	StoragePath  *string         `json:"storage_path,omitempty"`
	ErrorCode    *string         `json:"error_code,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	MetaJSON     json.RawMessage `json:"meta_json"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type RestoreRun struct {
	ID           int64           `json:"id"`
	ArtifactID   int64           `json:"artifact_id"`
	Status       RunStatus       `json:"status"`
	DryRun       bool            `json:"dry_run"`
	Steps        []RestoreStep   `json:"steps,omitempty"`
	SizeBytes    *int64          `json:"size_bytes,omitempty"`
	Filename     *string         `json:"filename,omitempty"`
	StoragePath  *string         `json:"storage_path,omitempty"`
	ErrorCode    *string         `json:"error_code,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	MetaJSON     json.RawMessage `json:"meta_json"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CreateBackupResult struct {
	Run      BackupRun      `json:"run"`
	Artifact BackupArtifact `json:"artifact"`
}

type ImportBackupRequest struct {
	File          io.ReadCloser
	OriginalName  string
	RequestedByID int64
}

type BackupPlan struct {
	ID                  int64      `json:"id"`
	Enabled             bool       `json:"enabled"`
	CronExpression      string     `json:"cron_expression"`
	ScheduleType        string     `json:"schedule_type,omitempty"`
	ScheduleWeekday     int        `json:"schedule_weekday,omitempty"`
	ScheduleMonthAnchor string     `json:"schedule_month_anchor,omitempty"`
	ScheduleHour        int        `json:"schedule_hour,omitempty"`
	ScheduleMinute      int        `json:"schedule_minute,omitempty"`
	RetentionDays       int        `json:"retention_days"`
	KeepLastSuccessful  int        `json:"keep_last_successful"`
	IncludeFiles        bool       `json:"include_files"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	LastAutoRunAt       *time.Time `json:"last_auto_run_at,omitempty"`
}

type CreateBackupOptions struct {
	Label        string   `json:"label,omitempty"`
	Scope        []string `json:"scope,omitempty"`
	IncludeFiles bool     `json:"include_files"`
	RequestedBy  string   `json:"-"`
}

type ListArtifactsFilter struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type DownloadArtifact struct {
	ID       int64
	Filename string
	Size     int64
	ModTime  time.Time
	Reader   ReadSeekCloser
}

type RestoreStep struct {
	Name           string         `json:"name"`
	Status         RunStatus      `json:"status"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	MessageI18NKey string         `json:"message_i18n_key"`
	Details        map[string]any `json:"details,omitempty"`
}
