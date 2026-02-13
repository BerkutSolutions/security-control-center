package backups

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"berkut-scc/config"
)

type concurrencyRepo struct {
	mu        sync.Mutex
	artifact  *BackupArtifact
	removedID int64
}

func (r *concurrencyRepo) ListArtifacts(ctx context.Context, filter ListArtifactsFilter) ([]BackupArtifact, error) {
	return nil, nil
}
func (r *concurrencyRepo) GetArtifact(ctx context.Context, id int64) (*BackupArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.artifact == nil || r.artifact.ID != id {
		return nil, ErrNotFound
	}
	cp := *r.artifact
	return &cp, nil
}
func (r *concurrencyRepo) ListSuccessfulArtifacts(ctx context.Context, limit int) ([]BackupArtifact, error) {
	return nil, nil
}
func (r *concurrencyRepo) DeleteArtifact(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.artifact == nil || r.artifact.ID != id {
		return sql.ErrNoRows
	}
	r.removedID = id
	r.artifact = nil
	return nil
}
func (r *concurrencyRepo) HasRunningBackupRun(ctx context.Context) (bool, error)  { return false, nil }
func (r *concurrencyRepo) HasRunningRestoreRun(ctx context.Context) (bool, error) { return false, nil }
func (r *concurrencyRepo) GetRestoreRun(ctx context.Context, id int64) (*RestoreRun, error) {
	return nil, ErrNotFound
}
func (r *concurrencyRepo) CreateRestoreRun(ctx context.Context, run *RestoreRun) (*RestoreRun, error) {
	return run, nil
}
func (r *concurrencyRepo) UpdateRestoreRun(ctx context.Context, run *RestoreRun) error { return nil }
func (r *concurrencyRepo) GetBackupPlan(ctx context.Context) (*BackupPlan, error) {
	return &BackupPlan{}, nil
}
func (r *concurrencyRepo) UpsertBackupPlan(ctx context.Context, plan *BackupPlan) (*BackupPlan, error) {
	return plan, nil
}
func (r *concurrencyRepo) SetBackupPlanEnabled(ctx context.Context, enabled bool) error { return nil }
func (r *concurrencyRepo) UpdateBackupPlanLastAutoRunAt(ctx context.Context, at time.Time) error {
	return nil
}
func (r *concurrencyRepo) GetMaintenanceMode(ctx context.Context) (bool, error) { return false, nil }
func (r *concurrencyRepo) SetMaintenanceMode(ctx context.Context, enabled bool, reason string) error {
	return nil
}
func (r *concurrencyRepo) CreateRun(ctx context.Context, run *BackupRun) (*BackupRun, error) {
	return run, nil
}
func (r *concurrencyRepo) UpdateRun(ctx context.Context, run *BackupRun) error { return nil }
func (r *concurrencyRepo) CreateArtifact(ctx context.Context, artifact *BackupArtifact) (*BackupArtifact, error) {
	return artifact, nil
}
func (r *concurrencyRepo) AttachArtifactToRun(ctx context.Context, runID, artifactID int64) error {
	return nil
}
func (r *concurrencyRepo) GetGooseVersion(ctx context.Context) (int64, error) { return 0, nil }
func (r *concurrencyRepo) GetArtifactByStoragePath(ctx context.Context, storagePath string) (*BackupArtifact, error) {
	return nil, ErrNotFound
}
func (r *concurrencyRepo) ResetRunningOperations(ctx context.Context) error { return nil }

func TestBeginRunBlocksConcurrentOperation(t *testing.T) {
	repo := &concurrencyRepo{}
	svc := NewService(&config.AppConfig{}, nil, repo, nil, nil)

	if err := svc.beginRun(context.Background()); err != nil {
		t.Fatalf("first beginRun failed: %v", err)
	}
	defer svc.endRun()

	err := svc.beginRun(context.Background())
	var de *DomainError
	if !errors.As(err, &de) || de.Code != ErrorCodeConcurrent {
		t.Fatalf("expected concurrent domain error, got %v", err)
	}
}

func TestStartRestoreBlockedWhileBackupPipelineActive(t *testing.T) {
	repo := &concurrencyRepo{}
	svc := NewService(&config.AppConfig{}, &sql.DB{}, repo, nil, nil)

	if err := svc.beginRun(context.Background()); err != nil {
		t.Fatalf("beginRun failed: %v", err)
	}
	defer svc.endRun()

	_, err := svc.startRestore(context.Background(), 10, "alice", false)
	var de *DomainError
	if !errors.As(err, &de) || de.Code != ErrorCodeConcurrent {
		t.Fatalf("expected concurrent domain error, got %v", err)
	}
}

func TestDeleteBackupBlockedDuringActiveDownload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.bscc")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write sample backup: %v", err)
	}
	size := int64(7)
	filename := "sample.bscc"
	storage := path
	status := StatusSuccess
	meta, _ := json.Marshal(map[string]string{"test": "1"})
	repo := &concurrencyRepo{
		artifact: &BackupArtifact{
			ID:          1,
			Status:      status,
			SizeBytes:   &size,
			Filename:    &filename,
			StoragePath: &storage,
			MetaJSON:    meta,
		},
	}
	cfg := &config.AppConfig{Backups: config.BackupsConfig{Path: dir}}
	svc := NewService(cfg, nil, repo, nil, nil)

	download, err := svc.DownloadArtifact(context.Background(), 1)
	if err != nil {
		t.Fatalf("download open failed: %v", err)
	}
	defer download.Reader.Close()

	err = svc.DeleteBackup(context.Background(), 1)
	var de *DomainError
	if !errors.As(err, &de) || de.Code != ErrorCodeFileBusy {
		t.Fatalf("expected file busy error, got %v", err)
	}
}
