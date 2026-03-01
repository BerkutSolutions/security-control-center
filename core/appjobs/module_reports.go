package appjobs

import (
	"context"
	"database/sql"

	"berkut-scc/core/appcompat"
)

func moduleReports(spec appcompat.ModuleSpec) Module {
	reportDocWhere := "doc_type='report'"
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}

				// Delete report-specific derived tables.
				r1, err := tx.ExecContext(ctx, `
					DELETE FROM report_snapshot_items
					WHERE snapshot_id IN (
						SELECT id FROM report_snapshots
						WHERE report_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)
					)`)
				if err == nil && r1 != nil {
					n, _ := r1.RowsAffected()
					counts["report_snapshot_items"] = n
				}
				r2, err := tx.ExecContext(ctx, `
					DELETE FROM report_snapshots
					WHERE report_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r2 != nil {
					n, _ := r2.RowsAffected()
					counts["report_snapshots"] = n
				}
				r3, err := tx.ExecContext(ctx, `
					DELETE FROM report_charts
					WHERE report_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r3 != nil {
					n, _ := r3.RowsAffected()
					counts["report_charts"] = n
				}
				r4, err := tx.ExecContext(ctx, `
					DELETE FROM report_sections
					WHERE report_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r4 != nil {
					n, _ := r4.RowsAffected()
					counts["report_sections"] = n
				}
				r5, err := tx.ExecContext(ctx, `
					DELETE FROM report_meta
					WHERE doc_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r5 != nil {
					n, _ := r5.RowsAffected()
					counts["report_meta"] = n
				}

				// Delete report docs and their versions/index/ACL.
				r6, err := tx.ExecContext(ctx, `
					DELETE FROM docs_fts
					WHERE doc_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r6 != nil {
					n, _ := r6.RowsAffected()
					counts["docs_fts.report_docs"] = n
				}
				r7, err := tx.ExecContext(ctx, `
					DELETE FROM entity_links
					WHERE doc_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r7 != nil {
					n, _ := r7.RowsAffected()
					counts["entity_links.report_docs"] = n
				}
				r8, err := tx.ExecContext(ctx, `
					DELETE FROM doc_acl
					WHERE doc_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r8 != nil {
					n, _ := r8.RowsAffected()
					counts["doc_acl.report_docs"] = n
				}
				r9, err := tx.ExecContext(ctx, `
					DELETE FROM doc_versions
					WHERE doc_id IN (SELECT id FROM docs WHERE `+reportDocWhere+`)`)
				if err == nil && r9 != nil {
					n, _ := r9.RowsAffected()
					counts["doc_versions.report_docs"] = n
				}
				r10, err := tx.ExecContext(ctx, `
					DELETE FROM docs
					WHERE `+reportDocWhere)
				if err != nil {
					return ModuleResult{}, err
				}
				if r10 != nil {
					n, _ := r10.RowsAffected()
					counts["docs.report_docs"] = n
				}

				// Reset report templates/settings.
				if exists, _ := tableExists(ctx, tx, "report_templates"); exists {
					n, err := deleteAll(ctx, tx, "report_templates")
					if err != nil {
						return ModuleResult{}, err
					}
					counts["report_templates"] = n
				}
				if exists, _ := tableExists(ctx, tx, "report_settings"); exists {
					n, err := deleteAll(ctx, tx, "report_settings")
					if err != nil {
						return ModuleResult{}, err
					}
					counts["report_settings"] = n
				}
				return ModuleResult{Counts: counts}, nil
			})
			if err != nil {
				return ModuleResult{}, err
			}
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return res, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				if exists, _ := tableExists(ctx, tx, "report_meta"); exists {
					n, err := deleteWhere(ctx, tx, "report_meta", "doc_id NOT IN (SELECT id FROM docs)")
					if err == nil && n > 0 {
						counts["report_meta.orphans_deleted"] = n
					}
				}
				if exists, _ := tableExists(ctx, tx, "report_sections"); exists {
					n, err := deleteWhere(ctx, tx, "report_sections", "report_id NOT IN (SELECT id FROM docs)")
					if err == nil && n > 0 {
						counts["report_sections.orphans_deleted"] = n
					}
				}
				if exists, _ := tableExists(ctx, tx, "report_charts"); exists {
					n, err := deleteWhere(ctx, tx, "report_charts", "report_id NOT IN (SELECT id FROM docs)")
					if err == nil && n > 0 {
						counts["report_charts.orphans_deleted"] = n
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

