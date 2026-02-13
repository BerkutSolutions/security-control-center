package backups

import (
	"context"
	"time"
)

type Repository interface {
	ListArtifacts(ctx context.Context, filter ListArtifactsFilter) ([]BackupArtifact, error)
	GetArtifact(ctx context.Context, id int64) (*BackupArtifact, error)
	ListSuccessfulArtifacts(ctx context.Context, limit int) ([]BackupArtifact, error)
	DeleteArtifact(ctx context.Context, id int64) error
	HasRunningBackupRun(ctx context.Context) (bool, error)
	HasRunningRestoreRun(ctx context.Context) (bool, error)
	GetRestoreRun(ctx context.Context, id int64) (*RestoreRun, error)
	CreateRestoreRun(ctx context.Context, run *RestoreRun) (*RestoreRun, error)
	UpdateRestoreRun(ctx context.Context, run *RestoreRun) error
	GetBackupPlan(ctx context.Context) (*BackupPlan, error)
	UpsertBackupPlan(ctx context.Context, plan *BackupPlan) (*BackupPlan, error)
	SetBackupPlanEnabled(ctx context.Context, enabled bool) error
	UpdateBackupPlanLastAutoRunAt(ctx context.Context, at time.Time) error
	GetMaintenanceMode(ctx context.Context) (bool, error)
	SetMaintenanceMode(ctx context.Context, enabled bool, reason string) error
	CreateRun(ctx context.Context, run *BackupRun) (*BackupRun, error)
	UpdateRun(ctx context.Context, run *BackupRun) error
	CreateArtifact(ctx context.Context, artifact *BackupArtifact) (*BackupArtifact, error)
	AttachArtifactToRun(ctx context.Context, runID, artifactID int64) error
	GetGooseVersion(ctx context.Context) (int64, error)
	GetArtifactByStoragePath(ctx context.Context, storagePath string) (*BackupArtifact, error)
	ResetRunningOperations(ctx context.Context) error
}

type ServicePort interface {
	ListArtifacts(ctx context.Context, filter ListArtifactsFilter) ([]BackupArtifact, error)
	GetArtifact(ctx context.Context, id int64) (*BackupArtifact, error)
	DownloadArtifact(ctx context.Context, id int64) (*DownloadArtifact, error)
	CreateBackup(ctx context.Context) (*CreateBackupResult, error)
	CreateBackupWithOptions(ctx context.Context, opts CreateBackupOptions) (*CreateBackupResult, error)
	ImportBackup(ctx context.Context, req ImportBackupRequest) (*BackupArtifact, error)
	RunAutoBackup(ctx context.Context) error
	DeleteBackup(ctx context.Context, id int64) error
	StartRestore(ctx context.Context, artifactID int64, requestedBy string) (*RestoreRun, error)
	StartRestoreDryRun(ctx context.Context, artifactID int64, requestedBy string) (*RestoreRun, error)
	GetRestoreRun(ctx context.Context, id int64) (*RestoreRun, error)
	GetPlan(ctx context.Context) (*BackupPlan, error)
	UpdatePlan(ctx context.Context, plan BackupPlan, requestedBy string) (*BackupPlan, error)
	EnablePlan(ctx context.Context, requestedBy string) (*BackupPlan, error)
	DisablePlan(ctx context.Context, requestedBy string) (*BackupPlan, error)
	IsMaintenanceMode(ctx context.Context) bool
	UploadMaxBytes() int64
}
