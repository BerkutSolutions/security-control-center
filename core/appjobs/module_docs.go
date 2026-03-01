package appjobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"

	"berkut-scc/core/appcompat"
)

func moduleDocs(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			dbRes, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"entity_links",
					"doc_acl",
					"doc_versions",
					"docs_fts",
					"docs",
					"folder_acl",
					"doc_templates",
					"doc_folders",
					"doc_reg_counters",
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
				storageDir = strings.TrimSpace(deps.Cfg.Docs.StorageDir)
			}
			if storageDir != "" {
				removed, rmErr := safeRemoveAllDir(filepath.Clean(storageDir))
				if rmErr != nil {
					return ModuleResult{}, rmErr
				}
				filesCounts["docs.storage.entries_removed"] = removed
			}
			dbRes.FilesCounts = filesCounts

			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return dbRes, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				// Cleanup invalid full-text index rows (reindex is expensive; avoid reading files).
				if exists, _ := tableExists(ctx, tx, "docs_fts"); exists {
					r, err := tx.ExecContext(ctx, `
						DELETE FROM docs_fts
						WHERE NOT EXISTS (
							SELECT 1 FROM doc_versions v
							WHERE v.doc_id=docs_fts.doc_id AND v.version=docs_fts.version_id
						)
					`)
					if err == nil && r != nil {
						n, _ := r.RowsAffected()
						counts["docs_fts.orphans"] = n
					}
				}
				// Normalize reg numbers.
				if exists, _ := tableExists(ctx, tx, "docs"); exists {
					r2, err := tx.ExecContext(ctx, `
						UPDATE docs
						SET reg_number=TRIM(reg_number)
						WHERE reg_number IS NOT NULL AND reg_number<>TRIM(reg_number)
					`)
					if err == nil && r2 != nil {
						n, _ := r2.RowsAffected()
						if n > 0 {
							counts["docs.reg_number_trimmed"] = n
						}
					}
				}
				// Remove empty entity links.
				if exists, _ := tableExists(ctx, tx, "entity_links"); exists {
					n, err := deleteWhere(ctx, tx, "entity_links", "TRIM(target_type)='' OR TRIM(target_id)=''")
					if err == nil && n > 0 {
						counts["entity_links.empty_deleted"] = n
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

