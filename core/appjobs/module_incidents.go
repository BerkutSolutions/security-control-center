package appjobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"berkut-scc/core/appcompat"
)

func moduleIncidents(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			dbRes, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"incident_artifact_files",
					"incident_timeline",
					"incident_attachments",
					"incident_links",
					"incident_acl",
					"incident_stage_entries",
					"incident_stages",
					"incident_participants",
					"incident_reg_counters",
					"incidents",
				}
				counts, err := deleteTablesInOrder(ctx, tx, tables)
				if err != nil {
					return ModuleResult{}, err
				}
				return ModuleResult{Counts: counts}, nil
			})
			if err != nil {
				return ModuleResult{}, err
			}

			filesCounts := map[string]int64{}
			storageDir := ""
			if deps.Cfg != nil {
				storageDir = strings.TrimSpace(deps.Cfg.Incidents.StorageDir)
			}
			if storageDir != "" {
				removed, rmErr := safeRemoveAllDir(filepath.Clean(storageDir))
				if rmErr != nil {
					return ModuleResult{}, rmErr
				}
				filesCounts["incidents.storage.entries_removed"] = removed
			}
			dbRes.FilesCounts = filesCounts

			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return dbRes, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				if exists, _ := tableExists(ctx, tx, "incidents"); exists {
					r, err := tx.ExecContext(ctx, `
						UPDATE incidents
						SET status=LOWER(TRIM(status))
						WHERE status IS NOT NULL AND status<>LOWER(TRIM(status))
					`)
					if err == nil && r != nil {
						n, _ := r.RowsAffected()
						if n > 0 {
							counts["incidents.status_normalized"] = n
						}
					}
				}
				if exists, _ := tableExists(ctx, tx, "incident_links"); exists {
					n, err := deleteWhere(ctx, tx, "incident_links", "TRIM(entity_type)='' OR TRIM(entity_id)=''")
					if err == nil && n > 0 {
						counts["incident_links.empty_deleted"] = n
					}
				}
				if exists, _ := tableExists(ctx, tx, "incident_timeline"); exists {
					n, err := deleteWhere(ctx, tx, "incident_timeline", "TRIM(event_type)='' OR TRIM(message)=''")
					if err == nil && n > 0 {
						counts["incident_timeline.empty_deleted"] = n
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

