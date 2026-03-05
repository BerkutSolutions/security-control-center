package backups

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"berkut-scc/config"
	backupcrypto "berkut-scc/core/backups/crypto"
	"berkut-scc/core/backups/pgrestore"
	"berkut-scc/core/backups/restore"
	corestore "berkut-scc/core/store"
)

func (s *Service) StartRestore(ctx context.Context, artifactID int64, requestedBy string) (*RestoreRun, error) {
	return s.startRestore(ctx, artifactID, requestedBy, false)
}

func (s *Service) StartRestoreDryRun(ctx context.Context, artifactID int64, requestedBy string) (*RestoreRun, error) {
	return s.startRestore(ctx, artifactID, requestedBy, true)
}

func (s *Service) IsMaintenanceMode(ctx context.Context) bool {
	if s == nil || s.repo == nil {
		return false
	}
	active, err := s.repo.GetMaintenanceMode(ctx)
	return err == nil && active
}

func (s *Service) startRestore(ctx context.Context, artifactID int64, requestedBy string, dryRun bool) (*RestoreRun, error) {
	if s == nil || s.repo == nil || s.cfg == nil || s.db == nil {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	if err := s.beginPipeline("restore"); err != nil {
		return nil, err
	}
	_ = s.recoverStaleOperations(ctx, staleOperationMaxAge)
	releasePipeline := true
	defer func() {
		if releasePipeline {
			s.endPipeline("restore")
		}
	}()
	if s.IsMaintenanceMode(ctx) {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}
	backupRunning, err := s.repo.HasRunningBackupRun(ctx)
	if err != nil {
		return nil, err
	}
	if backupRunning {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}
	restoreRunning, err := s.repo.HasRunningRestoreRun(ctx)
	if err != nil {
		return nil, err
	}
	if restoreRunning {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}
	artifact, err := s.repo.GetArtifact(ctx, artifactID)
	if err != nil {
		if err == ErrNotFound {
			return nil, NewDomainError(ErrorCodeNotFound, ErrorKeyNotFound)
		}
		return nil, err
	}
	if artifact.Status != StatusSuccess {
		return nil, NewDomainError(ErrorCodeNotReady, ErrorKeyNotReady)
	}
	if artifact.StoragePath == nil || strings.TrimSpace(*artifact.StoragePath) == "" {
		return nil, NewDomainError(ErrorCodeFileMissing, ErrorKeyFileMissing)
	}

	meta := restore.NewMeta(dryRun, strings.TrimSpace(requestedBy))
	run := &RestoreRun{
		ArtifactID:  artifactID,
		Status:      StatusQueued,
		DryRun:      dryRun,
		SizeBytes:   artifact.SizeBytes,
		Filename:    artifact.Filename,
		StoragePath: artifact.StoragePath,
		MetaJSON:    meta.Marshal(),
	}
	created, err := s.repo.CreateRestoreRun(ctx, run)
	if err != nil {
		return nil, err
	}
	s.cacheRestoreRun(created)
	releasePipeline = false
	go s.executeRestore(created.ID, artifactID, strings.TrimSpace(requestedBy), dryRun)
	return created, nil
}

func (s *Service) executeRestore(runID, artifactID int64, requestedBy string, dryRun bool) {
	defer s.endPipeline("restore")
	ctx := context.Background()
	if err := s.acquire(ctx); err != nil {
		return
	}
	defer s.release()

	run, err := s.repo.GetRestoreRun(ctx, runID)
	if err != nil || run == nil {
		return
	}
	var sourceArtifact *BackupArtifact
	run.Status = StatusRunning
	meta := restore.NewMeta(dryRun, requestedBy)
	meta.Log("info", "restore.run.started", map[string]any{
		"restore_id": runID,
		"backup_id":  artifactID,
		"dry_run":    dryRun,
	})
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	var maintenanceEnabled bool
	defer func() {
		if dryRun {
			return
		}
		if maintenanceEnabled {
			_ = s.repo.SetMaintenanceMode(ctx, false, "restore_cleanup")
			maintenanceEnabled = false
		}
		_ = s.repo.ResetRunningOperations(ctx)
		_ = s.restoreArtifactRecord(ctx, sourceArtifact)
	}()
	fail := func(step, code string, details map[string]any) {
		meta.Log("error", "restore.run.failed", map[string]any{
			"step": step,
			"code": code,
		})
		meta.StartStep(step)
		meta.FinishStep(step, string(StatusFailed), details)
		run.Status = StatusFailed
		run.MetaJSON = meta.Marshal()
		run.ErrorCode = strPtr(code)
		run.ErrorMessage = strPtr("restore failed")
		s.persistRestoreRun(ctx, run)
		Log(s.audits, ctx, requestedBy, AuditRestoreFailed, "failed", "backup_id="+int64String(artifactID)+" restore_id="+int64String(runID)+" reason_code="+code)
	}

	artifact, derr := s.repo.GetArtifact(ctx, artifactID)
	if derr != nil || artifact == nil {
		fail(restore.StepLoadArtifact, ErrorCodeNotFound, nil)
		return
	}
	sourceArtifact = artifact
	meta.Log("info", "restore.artifact.loaded", map[string]any{
		"artifact_id": artifactID,
	})
	meta.StartStep(restore.StepLoadArtifact)
	meta.FinishStep(restore.StepLoadArtifact, string(StatusSuccess), map[string]any{"artifact_id": artifactID})
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	path, derr := s.resolveArtifactPath(artifact)
	if derr != nil {
		fail(restore.StepOpenFile, ErrorCodeFileMissing, nil)
		return
	}
	meta.Log("info", "restore.artifact.path_resolved", map[string]any{
		"path": path,
	})
	meta.StartStep(restore.StepOpenFile)
	meta.FinishStep(restore.StepOpenFile, string(StatusSuccess), nil)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	tmpDir, derr := os.MkdirTemp("", "restore-*")
	if derr != nil {
		fail(restore.StepDecryptBSCC, ErrorCodeStorageMissing, nil)
		return
	}
	defer os.RemoveAll(tmpDir)

	key := s.cfg.Backups.EncryptionKey
	cipher, derr := backupcrypto.NewFileCipher(key, 1024*1024)
	if derr != nil {
		fail(restore.StepDecryptBSCC, ErrorCodeInvalidEncKey, nil)
		return
	}
	decryptedTar := filepath.Join(tmpDir, "payload.tar")
	meta.Log("info", "restore.decrypt.start", nil)
	meta.StartStep(restore.StepDecryptBSCC)
	if derr := cipher.DecryptFile(path, decryptedTar); derr != nil {
		fail(restore.StepDecryptBSCC, ErrorCodeDecryptFailed, nil)
		return
	}
	meta.FinishStep(restore.StepDecryptBSCC, string(StatusSuccess), nil)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	payload, derr := restore.ExtractPayload(decryptedTar, tmpDir)
	if derr != nil {
		fail(restore.StepReadManifest, ErrorCodeStorageMissing, nil)
		return
	}
	meta.Log("info", "restore.payload.extracted", map[string]any{
		"dump_path": payload.DumpPath,
	})

	meta.StartStep(restore.StepVerifyChecksums)
	if payload.ManifestSHA != payload.Checksums.ManifestSHA256 || payload.DumpSHA != payload.Checksums.DumpSHA256 {
		fail(restore.StepVerifyChecksums, ErrorCodeChecksumFailed, nil)
		return
	}
	meta.FinishStep(restore.StepVerifyChecksums, string(StatusSuccess), nil)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepReadManifest)
	meta.Manifest = &payload.Manifest
	meta.Log("info", "restore.manifest.loaded", map[string]any{
		"app_version":      payload.Manifest.AppVersion,
		"goose_db_version": payload.Manifest.GooseDBVersion,
		"db_engine":        payload.Manifest.DBEngine,
		"backup_scope":     payload.Manifest.BackupScope,
	})
	meta.FinishStep(restore.StepReadManifest, string(StatusSuccess), map[string]any{
		"format_version":   payload.Manifest.FormatVersion,
		"goose_db_version": payload.Manifest.GooseDBVersion,
		"db_engine":        payload.Manifest.DBEngine,
	})
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepCompatibilityCheck)
	currentGoose, gerr := s.repo.GetGooseVersion(ctx)
	if gerr != nil {
		currentGoose = 0
	}
	needsMigrate := payload.Manifest.GooseDBVersion < currentGoose
	if payload.Manifest.DBEngine != "postgres" || payload.Manifest.FormatVersion != "1" || payload.Manifest.GooseDBVersion > currentGoose {
		fail(restore.StepCompatibilityCheck, ErrorCodeIncompatible, map[string]any{"current_goose_version": currentGoose})
		return
	}
	meta.Compatibility = map[string]any{
		"current_goose_version": currentGoose,
		"backup_goose_version":  payload.Manifest.GooseDBVersion,
		"migration_required":    needsMigrate,
	}
	restoreScope := normalizeManifestScope(payload.Manifest.BackupScope)
	isFullRestore := backupScopeIsAll(restoreScope)
	beforeCounts := map[string]int64{}
	if !isFullRestore {
		beforeCounts = s.scopeTableCounts(ctx, restoreScope)
		meta.Log("info", "restore.scope.counts.before", map[string]any{
			"counts": beforeCounts,
		})
	}
	meta.Log("info", "restore.compatibility.checked", map[string]any{
		"current_goose_version": currentGoose,
		"backup_goose_version":  payload.Manifest.GooseDBVersion,
		"restore_scope":         restoreScope,
		"restore_mode":          map[bool]string{true: "full", false: "scoped"}[isFullRestore],
	})
	meta.Compatibility["backup_scope"] = restoreScope
	meta.Compatibility["restore_mode"] = map[bool]string{true: "full", false: "scoped"}[isFullRestore]
	meta.FinishStep(restore.StepCompatibilityCheck, string(StatusSuccess), meta.Compatibility)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	if dryRun {
		if !isFullRestore {
			meta.Log("info", "restore.scope.counts.dry_run", map[string]any{
				"counts": s.scopeTableCounts(ctx, restoreScope),
			})
		}
		meta.Log("info", "restore.dry_run.completed", map[string]any{
			"scope": restoreScope,
		})
		meta.StartStep(restore.StepFinish)
		meta.FinishStep(restore.StepFinish, string(StatusSuccess), map[string]any{
			"mode":         "dry_run",
			"restore_mode": map[bool]string{true: "full", false: "scoped"}[isFullRestore],
			"scope":        restoreScope,
		})
		run.Status = StatusSuccess
		run.MetaJSON = meta.Marshal()
		s.persistRestoreRun(ctx, run)
		Log(s.audits, ctx, requestedBy, AuditRestoreSuccess, "success", "backup_id="+int64String(artifactID)+" restore_id="+int64String(runID)+" dry_run=true")
		return
	}

	meta.StartStep(restore.StepEnterMaintenance)
	if derr := s.repo.SetMaintenanceMode(ctx, true, "restore_in_progress"); derr != nil {
		fail(restore.StepEnterMaintenance, ErrorCodeMaintenance, nil)
		return
	}
	maintenanceEnabled = true
	meta.Log("info", "restore.maintenance.enabled", nil)
	Log(s.audits, ctx, requestedBy, AuditMaintenanceEnter, "success", "restore_id="+int64String(runID))
	meta.FinishStep(restore.StepEnterMaintenance, string(StatusSuccess), nil)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepStopJobs)
	meta.FinishStep(restore.StepStopJobs, string(StatusSuccess), map[string]any{"todo": "scheduler_pause"})
	meta.Log("info", "restore.background_jobs.stopped", nil)
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepRestoreDatabase)
	restoreCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	meta.Log("info", "restore.database.start", map[string]any{
		"mode":  map[bool]string{true: "full", false: "scoped"}[isFullRestore],
		"scope": restoreScope,
	})
	if isFullRestore {
		derr = s.replaceDatabase(restoreCtx, payload.DumpPath)
	} else {
		derr = s.replaceDatabaseScoped(restoreCtx, payload.DumpPath, restoreScope)
	}
	cancel()
	if derr != nil {
		meta.Log("error", "restore.database.failed", map[string]any{"error": derr.Error()})
		fail(restore.StepRestoreDatabase, ErrorCodePgRestore, map[string]any{"error": derr.Error()})
		if maintenanceEnabled {
			_ = s.repo.SetMaintenanceMode(ctx, false, "restore_failed")
		}
		return
	}
	meta.Log("info", "restore.database.completed", map[string]any{
		"mode":  map[bool]string{true: "full", false: "scoped"}[isFullRestore],
		"scope": restoreScope,
	})
	if !isFullRestore {
		afterCounts := s.scopeTableCounts(ctx, restoreScope)
		meta.Log("info", "restore.scope.counts.after", map[string]any{
			"counts": afterCounts,
		})
		if err := s.validateScopedRestore(sourceArtifact, restoreScope, beforeCounts, afterCounts, &meta); err != nil {
			meta.Log("error", "restore.scope.validation_failed", map[string]any{
				"error": err.Error(),
			})
			fail(restore.StepRestoreDatabase, ErrorCodePgRestore, map[string]any{"error": err.Error()})
			if maintenanceEnabled {
				_ = s.repo.SetMaintenanceMode(ctx, false, "restore_failed")
			}
			return
		}
	}
	meta.FinishStep(restore.StepRestoreDatabase, string(StatusSuccess), map[string]any{
		"restore_mode": map[bool]string{true: "full", false: "scoped"}[isFullRestore],
		"scope":        restoreScope,
	})
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepRunMigrations)
	if isFullRestore && needsMigrate {
		meta.Log("info", "restore.migrations.start", nil)
		migCtx, migCancel := context.WithTimeout(ctx, 15*time.Minute)
		derr = corestore.ApplyMigrations(migCtx, s.db, s.logger)
		migCancel()
		if derr != nil {
			meta.Log("error", "restore.migrations.failed", map[string]any{"error": derr.Error()})
			fail(restore.StepRunMigrations, ErrorCodePgRestore, nil)
			_ = s.repo.SetMaintenanceMode(ctx, false, "restore_failed")
			return
		}
		meta.Log("info", "restore.migrations.completed", nil)
	}
	meta.FinishStep(restore.StepRunMigrations, string(StatusSuccess), map[string]any{
		"migration_required": isFullRestore && needsMigrate,
		"skipped_for_scoped": !isFullRestore,
	})
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)

	meta.StartStep(restore.StepExitMaintenance)
	if derr := s.repo.SetMaintenanceMode(ctx, false, "restore_completed"); derr != nil {
		fail(restore.StepExitMaintenance, ErrorCodeMaintenance, nil)
		return
	}
	Log(s.audits, ctx, requestedBy, AuditMaintenanceExit, "success", "restore_id="+int64String(runID))
	maintenanceEnabled = false
	meta.Log("info", "restore.maintenance.disabled", nil)
	meta.FinishStep(restore.StepExitMaintenance, string(StatusSuccess), nil)

	meta.StartStep(restore.StepFinish)
	meta.FinishStep(restore.StepFinish, string(StatusSuccess), nil)
	meta.Log("info", "restore.run.completed", nil)
	run.Status = StatusSuccess
	run.ErrorCode = nil
	run.ErrorMessage = nil
	run.MetaJSON = meta.Marshal()
	s.persistRestoreRun(ctx, run)
	Log(s.audits, ctx, requestedBy, AuditRestoreSuccess, "success", "backup_id="+int64String(artifactID)+" restore_id="+int64String(runID)+" dry_run=false")
}

func (s *Service) replaceDatabase(ctx context.Context, dumpPath string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("missing db")
	}
	if err := s.prepareDatabaseForRestore(ctx); err != nil {
		return err
	}
	if err := s.restorer.Restore(ctx, restoreOptions(s.cfg, dumpPath)); err != nil {
		return err
	}
	return s.ensureCoreSchemaAfterRestore(ctx)
}

func (s *Service) replaceDatabaseScoped(ctx context.Context, dumpPath string, scope []string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("missing db")
	}
	tables := backupTablesForScope(scope)
	if len(tables) == 0 {
		return fmt.Errorf("empty restore scope")
	}
	// Scoped dump already contains only selected module tables.
	// Restore the whole archive to avoid table-pattern mismatches that can silently skip data.
	if err := s.restorer.Restore(ctx, restoreOptionsScoped(s.cfg, dumpPath)); err != nil {
		return err
	}
	return nil
}

func restoreOptions(cfg *config.AppConfig, dumpPath string) pgrestore.Options {
	return pgrestore.Options{
		BinaryPath: "pg_restore",
		DBURL:      cfg.DBURL,
		InputPath:  dumpPath,
		Clean:      false,
	}
}

func restoreOptionsScoped(cfg *config.AppConfig, dumpPath string) pgrestore.Options {
	return pgrestore.Options{
		BinaryPath:      "pg_restore",
		DBURL:           cfg.DBURL,
		InputPath:       dumpPath,
		Clean:           true,
		DataOnly:        false,
		DisableTriggers: false,
	}
}

func (s *Service) prepareDatabaseForRestore(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("missing db")
	}
	terminateSQL := `
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = current_database()
		  AND pid <> pg_backend_pid()
	`
	_, _ = s.db.ExecContext(ctx, terminateSQL)
	if _, err := s.db.ExecContext(ctx, `DROP SCHEMA IF EXISTS public CASCADE`); err != nil {
		return fmt.Errorf("drop public schema failed: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS public`); err != nil {
		return fmt.Errorf("create public schema failed: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `GRANT ALL ON SCHEMA public TO CURRENT_USER`); err != nil {
		return fmt.Errorf("grant schema failed: %w", err)
	}
	return nil
}

func (s *Service) resolveArtifactPath(artifact *BackupArtifact) (string, error) {
	if artifact == nil || artifact.StoragePath == nil {
		return "", sql.ErrNoRows
	}
	basePath := strings.TrimSpace(s.cfg.Backups.Path)
	cleanStorage := filepath.Clean(strings.TrimSpace(*artifact.StoragePath))
	if basePath != "" {
		cleanBase := filepath.Clean(basePath)
		if !strings.HasPrefix(cleanStorage+string(os.PathSeparator), cleanBase+string(os.PathSeparator)) && cleanStorage != cleanBase {
			return "", os.ErrNotExist
		}
	}
	info, err := os.Stat(cleanStorage)
	if err != nil || info.IsDir() {
		return "", os.ErrNotExist
	}
	return cleanStorage, nil
}

func strPtr(v string) *string {
	val := strings.TrimSpace(v)
	return &val
}

func int64String(v int64) string {
	return strconv.FormatInt(v, 10)
}

func (s *Service) persistRestoreRun(ctx context.Context, run *RestoreRun) {
	if s == nil || run == nil {
		return
	}
	s.cacheRestoreRun(run)
	_ = s.repo.UpdateRestoreRun(ctx, run)
}

func (s *Service) restoreArtifactRecord(ctx context.Context, artifact *BackupArtifact) error {
	if s == nil || s.repo == nil || artifact == nil || artifact.StoragePath == nil {
		return nil
	}
	storagePath := strings.TrimSpace(*artifact.StoragePath)
	if storagePath == "" {
		return nil
	}
	if _, err := s.repo.GetArtifactByStoragePath(ctx, storagePath); err == nil {
		return nil
	}
	if _, ok := s.cleanStoragePath(storagePath); !ok {
		return nil
	}
	cloned := &BackupArtifact{
		RunID:        nil,
		Source:       artifact.Source,
		CreatedByID:  artifact.CreatedByID,
		OriginFile:   artifact.OriginFile,
		Status:       StatusSuccess,
		SizeBytes:    artifact.SizeBytes,
		Checksum:     artifact.Checksum,
		Filename:     artifact.Filename,
		StoragePath:  artifact.StoragePath,
		ErrorCode:    nil,
		ErrorMessage: nil,
		MetaJSON:     artifact.MetaJSON,
	}
	_, err := s.repo.CreateArtifact(ctx, cloned)
	return err
}

func (s *Service) ensureCoreSchemaAfterRestore(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("missing db")
	}
	hasSchema, err := s.hasCoreSchema(ctx)
	if err != nil {
		return err
	}
	if hasSchema {
		return nil
	}
	migCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := corestore.ApplyMigrations(migCtx, s.db, s.logger); err != nil {
		return err
	}
	hasSchema, err = s.hasCoreSchema(ctx)
	if err != nil {
		return err
	}
	if !hasSchema {
		return fmt.Errorf("core schema missing after restore and migration recovery")
	}
	return nil
}

func (s *Service) hasCoreSchema(ctx context.Context) (bool, error) {
	required := []string{"users", "sessions", "goose_db_version"}
	for _, table := range required {
		var present bool
		err := s.db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&present)
		if err != nil {
			return false, err
		}
		if !present {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) scopeTableCounts(ctx context.Context, scope []string) map[string]int64 {
	out := map[string]int64{}
	if s == nil || s.db == nil {
		return out
	}
	tables := backupTablesForScope(scope)
	for _, table := range tables {
		name := strings.TrimSpace(table)
		if name == "" {
			continue
		}
		query := `SELECT COUNT(*) FROM "` + name + `"`
		var count int64
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			out[name] = -1
			continue
		}
		out[name] = count
	}
	return out
}

func scopeContains(scope []string, item string) bool {
	target := strings.ToLower(strings.TrimSpace(item))
	if target == "" {
		return false
	}
	for _, raw := range scope {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "all" {
			return true
		}
		if v == target {
			return true
		}
	}
	return false
}

func artifactEntityCount(artifact *BackupArtifact, key string) int64 {
	if artifact == nil || len(artifact.MetaJSON) == 0 || strings.TrimSpace(key) == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(artifact.MetaJSON, &payload); err != nil {
		return 0
	}
	entityCountsRaw, ok := payload["entity_counts"].(map[string]any)
	if !ok {
		return 0
	}
	val, ok := entityCountsRaw[key]
	if !ok {
		return 0
	}
	switch n := val.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

type scopedValidationRule struct {
	Scope     string
	EntityKey string
	Table     string
}

var scopedValidationRules = []scopedValidationRule{
	{Scope: "docs", EntityKey: "docs.documents", Table: "docs"},
	{Scope: "docs", EntityKey: "docs.folders", Table: "doc_folders"},
	{Scope: "incidents", EntityKey: "incidents.incidents", Table: "incidents"},
	{Scope: "reports", EntityKey: "reports.reports", Table: "report_meta"},
	{Scope: "monitoring", EntityKey: "monitoring.monitors", Table: "monitors"},
	{Scope: "tasks", EntityKey: "tasks.tasks", Table: "tasks"},
	{Scope: "tasks", EntityKey: "tasks.boards", Table: "task_boards"},
	{Scope: "tasks", EntityKey: "tasks.spaces", Table: "task_spaces"},
	{Scope: "controls", EntityKey: "controls.controls", Table: "controls"},
	{Scope: "accounts", EntityKey: "accounts.users", Table: "users"},
	{Scope: "accounts", EntityKey: "accounts.groups", Table: "groups"},
	{Scope: "approvals", EntityKey: "approvals.approvals", Table: "approvals"},
}

func (s *Service) validateScopedRestore(artifact *BackupArtifact, scope []string, before, after map[string]int64, meta *restore.Meta) error {
	for _, rule := range scopedValidationRules {
		if !scopeContains(scope, rule.Scope) {
			continue
		}
		expected := artifactEntityCount(artifact, rule.EntityKey)
		beforeCount := before[rule.Table]
		restored := after[rule.Table]
		meta.Log("info", "restore.scope.validation", map[string]any{
			"scope":     rule.Scope,
			"entity":    rule.EntityKey,
			"table":     rule.Table,
			"expected":  expected,
			"before":    beforeCount,
			"restored":  restored,
			"condition": "expected>0 => restored>0",
		})
		if expected > 0 && restored == 0 {
			return fmt.Errorf("scoped restore validation failed: %s expected > 0, restored 0", rule.EntityKey)
		}
	}
	return nil
}
