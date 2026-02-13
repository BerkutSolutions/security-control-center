package backups

import (
	"context"
	"os"
	"strings"
	"time"
)

func (s *Service) RunAutoBackup(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return NewDomainError(ErrorCodeInternal, "common.serverError")
	}
	if s.IsMaintenanceMode(ctx) {
		return nil
	}
	backupRunning, err := s.repo.HasRunningBackupRun(ctx)
	if err != nil {
		return err
	}
	if backupRunning {
		return nil
	}
	restoreRunning, err := s.repo.HasRunningRestoreRun(ctx)
	if err != nil {
		return err
	}
	if restoreRunning {
		return nil
	}
	Log(s.audits, ctx, "scheduler", AuditAutoStarted, "started", "event=backups.auto.started")
	backupCtx, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	result, err := s.CreateBackupWithOptions(backupCtx, CreateBackupOptions{
		Label:        "AUTO",
		Scope:        []string{"ALL"},
		IncludeFiles: planIncludeFiles(ctx, s),
		RequestedBy:  "scheduler",
	})
	if err != nil {
		code := ErrorCodeInternal
		if de, ok := AsDomainError(err); ok {
			code = de.Code
		}
		Log(s.audits, ctx, "scheduler", AuditAutoFailed, "failed", "event=backups.auto.failed reason_code="+code)
		return err
	}
	plan, err := s.GetPlan(ctx)
	if err == nil && plan != nil {
		_ = s.applyRetention(ctx, *plan)
		_ = s.repo.UpdateBackupPlanLastAutoRunAt(ctx, time.Now().UTC())
	}
	details := "event=backups.auto.success"
	if result != nil && result.Artifact.Filename != nil {
		details += " filename=" + sanitizeAuditValue(*result.Artifact.Filename)
	}
	Log(s.audits, ctx, "scheduler", AuditAutoSuccess, "success", details)
	return nil
}

func planIncludeFiles(ctx context.Context, s *Service) bool {
	if s == nil {
		return false
	}
	plan, err := s.GetPlan(ctx)
	if err != nil || plan == nil {
		return false
	}
	return plan.IncludeFiles
}

func (s *Service) applyRetention(ctx context.Context, plan BackupPlan) error {
	items, err := s.repo.ListSuccessfulArtifacts(ctx, 1000)
	if err != nil || len(items) == 0 {
		return err
	}
	keep := plan.KeepLastSuccessful
	if keep <= 0 {
		keep = 5
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -max(plan.RetentionDays, 1))
	for idx := range items {
		item := items[idx]
		if idx < keep {
			continue
		}
		if !item.CreatedAt.Before(cutoff) {
			continue
		}
		if item.StoragePath != nil && strings.TrimSpace(*item.StoragePath) != "" {
			_ = os.Remove(strings.TrimSpace(*item.StoragePath))
		}
		if err := s.repo.DeleteArtifact(ctx, item.ID); err != nil {
			continue
		}
		Log(s.audits, ctx, "scheduler", AuditRetentionDeleted, "success", "event=backups.retention.deleted backup_id="+int64String(item.ID))
	}
	return nil
}

func sanitizeAuditValue(in string) string {
	v := strings.TrimSpace(in)
	v = strings.ReplaceAll(v, " ", "_")
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
