package appjobs

import (
	"context"
	"database/sql"
	"path/filepath"

	"berkut-scc/core/appcompat"
)

func moduleTasks(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			dbRes, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"task_tag_links",
					"task_files",
					"task_blocks",
					"task_comments",
					"task_assignments",
					"task_archive_entries",
					"tasks",
					"task_subcolumns",
					"task_columns",
					"task_board_acl",
					"task_recurring_instances",
					"task_recurring_rules",
					"task_templates",
					"task_board_layouts",
					"task_boards",
					"task_space_acl",
					"task_spaces",
					"task_tags",
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

			// Remove stored task files.
			filesCounts := map[string]int64{}
			removed, rmErr := safeRemoveAllDir(filepath.Join("data", "tasks"))
			if rmErr != nil {
				return ModuleResult{}, rmErr
			}
			filesCounts["data/tasks.entries_removed"] = removed
			dbRes.FilesCounts = filesCounts

			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return dbRes, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				// Best-effort cleanup of potential orphans (should be prevented by FKs).
				if n, err := deleteWhere(ctx, tx, "task_tag_links", "task_id NOT IN (SELECT id FROM tasks)"); err == nil {
					counts["task_tag_links.orphans"] = n
				}
				if n, err := deleteWhere(ctx, tx, "task_files", "task_id NOT IN (SELECT id FROM tasks)"); err == nil {
					counts["task_files.orphans"] = n
				}
				if n, err := deleteWhere(ctx, tx, "task_comments", "task_id NOT IN (SELECT id FROM tasks)"); err == nil {
					counts["task_comments.orphans"] = n
				}
				if n, err := deleteWhere(ctx, tx, "task_assignments", "task_id NOT IN (SELECT id FROM tasks)"); err == nil {
					counts["task_assignments.orphans"] = n
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

