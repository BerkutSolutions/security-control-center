package appjobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	"berkut-scc/core/appcompat"
)

func moduleBackups(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			dbRes, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"backups_restore_runs",
					"backups_runs",
					"backups_artifacts",
					"backup_plans",
					"app_maintenance_state",
				}
				counts, err := deleteTablesInOrder(ctx, tx, tables)
				if err != nil {
					return ModuleResult{}, err
				}
				now := deps.NowUTC
				if now.IsZero() {
					now = time.Now().UTC()
				}
				// Restore default rows.
				if exists, _ := tableExists(ctx, tx, "backup_plans"); exists {
					_, _ = tx.ExecContext(ctx, `
						INSERT INTO backup_plans(id, enabled, cron_expression, retention_days, keep_last_successful, include_files, created_at, updated_at)
						VALUES(1, FALSE, '0 2 * * *', 30, 5, FALSE, ?, ?)
						ON CONFLICT (id) DO NOTHING
					`, now.UTC(), now.UTC())
					counts["backup_plans.default_inserted"] = 1
				}
				if exists, _ := tableExists(ctx, tx, "app_maintenance_state"); exists {
					_, _ = tx.ExecContext(ctx, `
						INSERT INTO app_maintenance_state(id, enabled, reason, updated_at)
						VALUES(1, FALSE, '', ?)
						ON CONFLICT (id) DO NOTHING
					`, now.UTC())
					counts["app_maintenance_state.default_inserted"] = 1
				}
				return ModuleResult{Counts: counts}, nil
			})
			if err != nil {
				return ModuleResult{}, err
			}
			filesCounts := map[string]int64{}
			base := ""
			if deps.Cfg != nil {
				base = strings.TrimSpace(deps.Cfg.Backups.Path)
			}
			if base != "" {
				removed, rmErr := safeRemoveAllDir(filepath.Clean(base))
				if rmErr != nil {
					return ModuleResult{}, rmErr
				}
				filesCounts["backups.storage.entries_removed"] = removed
			}
			dbRes.FilesCounts = filesCounts

			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return dbRes, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				if exists, _ := tableExists(ctx, tx, "backups_runs"); exists {
					r, err := tx.ExecContext(ctx, `
						UPDATE backups_runs
						SET status=LOWER(TRIM(status))
						WHERE status IS NOT NULL AND status<>LOWER(TRIM(status))
					`)
					if err == nil && r != nil {
						n, _ := r.RowsAffected()
						if n > 0 {
							counts["backups_runs.status_normalized"] = n
						}
					}
				}
				if len(counts) == 0 {
					counts["noop"] = 1
				}
				return ModuleResult{Counts: counts}, nil
			})
			if err != nil {
				return ModuleResult{}, err
			}
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return res, nil
		},
	}
}

