package backups

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type IntegrityStatus string

const (
	IntegrityStatusOK      IntegrityStatus = "ok"
	IntegrityStatusWarning IntegrityStatus = "warning"
	IntegrityStatusFailed  IntegrityStatus = "failed"
)

type BackupIntegrityStatus struct {
	Status                   IntegrityStatus `json:"status"`
	Enabled                  bool            `json:"enabled"`
	IntervalHours            int             `json:"interval_hours"`
	LastSuccessfulBackupID   *int64          `json:"last_successful_backup_id,omitempty"`
	LastSuccessfulBackupAt   *time.Time      `json:"last_successful_backup_at,omitempty"`
	LastRestoreTestID        *int64          `json:"last_restore_test_id,omitempty"`
	LastRestoreTestBackupID  *int64          `json:"last_restore_test_backup_id,omitempty"`
	LastRestoreTestAt        *time.Time      `json:"last_restore_test_at,omitempty"`
	LastRestoreTestStatus    string          `json:"last_restore_test_status,omitempty"`
	LastSuccessfulTestAt     *time.Time      `json:"last_successful_test_at,omitempty"`
	LastFailedTestAt         *time.Time      `json:"last_failed_test_at,omitempty"`
	NextScheduledTestAt      *time.Time      `json:"next_scheduled_test_at,omitempty"`
	Overdue                  bool            `json:"overdue"`
	LastErrorI18NKey         string          `json:"last_error_i18n_key,omitempty"`
	SchedulerTriggeredBy     string          `json:"scheduler_triggered_by,omitempty"`
	SchedulerLastTriggeredAt *time.Time      `json:"scheduler_last_triggered_at,omitempty"`
}

type restoreRunSnapshot struct {
	ID         int64
	ArtifactID int64
	Status     RunStatus
	CreatedAt  time.Time
	DryRun     bool
}

func (s *Service) IntegrityStatus(ctx context.Context) (*BackupIntegrityStatus, error) {
	if s == nil || s.repo == nil || s.cfg == nil {
		return &BackupIntegrityStatus{
			Status:        IntegrityStatusWarning,
			Enabled:       false,
			IntervalHours: 0,
			Overdue:       false,
		}, nil
	}

	items, err := s.repo.ListSuccessfulArtifacts(ctx, 1)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.listRestoreRunSnapshots(ctx, 50)
	if err != nil {
		return nil, err
	}

	interval := s.restoreTestInterval()
	intervalHours := int(interval / time.Hour)
	now := time.Now().UTC()
	status := &BackupIntegrityStatus{
		Status:        IntegrityStatusWarning,
		Enabled:       s.cfg.Backups.RestoreTestAutoEnabled,
		IntervalHours: intervalHours,
		Overdue:       false,
	}

	if len(items) == 0 {
		status.LastErrorI18NKey = "backups.integrity.noBackups"
		return status, nil
	}

	latestBackup := items[0]
	status.LastSuccessfulBackupID = &latestBackup.ID
	status.LastSuccessfulBackupAt = timePtr(latestBackup.CreatedAt.UTC())

	var latestDryRun *restoreRunSnapshot
	for i := range snapshots {
		if !snapshots[i].DryRun {
			continue
		}
		if latestDryRun == nil || snapshots[i].CreatedAt.After(latestDryRun.CreatedAt) {
			snapshot := snapshots[i]
			latestDryRun = &snapshot
		}
	}

	if latestDryRun != nil {
		status.LastRestoreTestID = &latestDryRun.ID
		status.LastRestoreTestBackupID = &latestDryRun.ArtifactID
		status.LastRestoreTestAt = timePtr(latestDryRun.CreatedAt.UTC())
		status.LastRestoreTestStatus = string(latestDryRun.Status)
		next := latestDryRun.CreatedAt.UTC().Add(interval)
		status.NextScheduledTestAt = timePtr(next)
		status.Overdue = now.After(next)
	} else if status.LastSuccessfulBackupAt != nil {
		next := status.LastSuccessfulBackupAt.Add(interval)
		status.NextScheduledTestAt = timePtr(next)
		status.Overdue = now.After(next)
	}

	for i := range snapshots {
		if !snapshots[i].DryRun {
			continue
		}
		if snapshots[i].Status == StatusSuccess {
			status.LastSuccessfulTestAt = timePtr(snapshots[i].CreatedAt.UTC())
			break
		}
	}
	for i := range snapshots {
		if !snapshots[i].DryRun {
			continue
		}
		if snapshots[i].Status == StatusFailed {
			status.LastFailedTestAt = timePtr(snapshots[i].CreatedAt.UTC())
			break
		}
	}

	status.Status = IntegrityStatusWarning
	switch {
	case latestDryRun == nil:
		status.LastErrorI18NKey = "backups.integrity.restoreTestMissing"
	case latestDryRun.Status == StatusFailed:
		status.Status = IntegrityStatusFailed
		status.LastErrorI18NKey = "backups.integrity.restoreTestFailed"
	case latestDryRun.Status == StatusSuccess && !status.Overdue:
		status.Status = IntegrityStatusOK
	default:
		status.Status = IntegrityStatusWarning
		status.LastErrorI18NKey = "backups.integrity.restoreTestStale"
	}

	if status.Overdue && status.LastErrorI18NKey == "" {
		status.LastErrorI18NKey = "backups.integrity.restoreTestStale"
	}
	return status, nil
}

func (s *Service) StartIntegrityVerification(ctx context.Context, requestedBy string) (*RestoreRun, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeInternal, ErrorKeyInternal)
	}
	successful, err := s.repo.ListSuccessfulArtifacts(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(successful) == 0 {
		return nil, NewDomainError(ErrorCodeNotReady, "backups.integrity.noBackups")
	}
	return s.StartRestoreDryRun(ctx, successful[0].ID, requestedBy)
}

func (s *Service) RunAutoRestoreTest(ctx context.Context, now time.Time) error {
	if s == nil || s.cfg == nil || s.repo == nil || !s.cfg.Backups.RestoreTestAutoEnabled {
		return nil
	}
	info, err := s.IntegrityStatus(ctx)
	if err != nil || info == nil {
		return err
	}
	if !info.Overdue || info.LastSuccessfulBackupID == nil {
		return nil
	}

	backupRunning, err := s.repo.HasRunningBackupRun(ctx)
	if err != nil || backupRunning {
		return err
	}
	restoreRunning, err := s.repo.HasRunningRestoreRun(ctx)
	if err != nil || restoreRunning || s.IsMaintenanceMode(ctx) {
		return err
	}

	requestedBy := "scheduler:restore-test"
	_, err = s.StartRestoreDryRun(ctx, *info.LastSuccessfulBackupID, requestedBy)
	if err != nil {
		Log(s.audits, ctx, requestedBy, AuditRestoreDryRunAuto, "failed", "reason=queue_failed")
		return err
	}
	Log(s.audits, ctx, requestedBy, AuditRestoreDryRunAuto, "queued", "backup_id="+int64String(*info.LastSuccessfulBackupID))
	_ = now
	return nil
}

func (s *Service) listRestoreRunSnapshots(ctx context.Context, limit int) ([]restoreRunSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, artifact_id, status, meta_json, created_at
		FROM backups_restore_runs
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]restoreRunSnapshot, 0, limit)
	for rows.Next() {
		var item restoreRunSnapshot
		var metaRaw []byte
		if err := rows.Scan(&item.ID, &item.ArtifactID, &item.Status, &metaRaw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.DryRun = decodeRestoreRunMode(metaRaw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func decodeRestoreRunMode(metaRaw []byte) bool {
	if len(metaRaw) == 0 {
		return false
	}
	meta := struct {
		Mode string `json:"mode"`
	}{}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(meta.Mode), "dry_run")
}

func restoreTestInterval() time.Duration {
	return 7 * 24 * time.Hour
}

func (s *Service) restoreTestInterval() time.Duration {
	if s == nil || s.cfg == nil || s.cfg.Backups.RestoreTestIntervalHours <= 0 {
		return restoreTestInterval()
	}
	interval := time.Duration(s.cfg.Backups.RestoreTestIntervalHours) * time.Hour
	if interval < 24*time.Hour {
		return 24 * time.Hour
	}
	return interval
}

func timePtr(t time.Time) *time.Time {
	v := t.UTC()
	return &v
}
