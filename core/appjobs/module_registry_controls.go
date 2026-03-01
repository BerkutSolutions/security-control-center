package appjobs

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"berkut-scc/core/appcompat"
	"berkut-scc/core/controls"
)

func moduleRegistryControls(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"control_framework_map",
					"control_framework_items",
					"control_frameworks",
					"control_violations",
					"control_comments",
					"control_checks",
					"controls",
					"control_types",
				}
				counts, err := deleteTablesInOrder(ctx, tx, tables)
				if err != nil {
					return ModuleResult{}, err
				}
				// Restore built-in control types.
				now := deps.NowUTC
				if now.IsZero() {
					now = time.Now().UTC()
				}
				var restored int64
				for _, raw := range controls.ControlTypes {
					name := strings.TrimSpace(raw)
					if name == "" {
						continue
					}
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO control_types(name, is_builtin, created_at)
						VALUES(?, 1, ?)
						ON CONFLICT (name) DO NOTHING
					`, name, now.UTC()); err != nil {
						return ModuleResult{}, err
					}
					restored++
				}
				counts["control_types.builtin_restored"] = restored
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
				now := deps.NowUTC
				if now.IsZero() {
					now = time.Now().UTC()
				}
				var restored int64
				for _, raw := range controls.ControlTypes {
					name := strings.TrimSpace(raw)
					if name == "" {
						continue
					}
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO control_types(name, is_builtin, created_at)
						VALUES(?, 1, ?)
						ON CONFLICT (name) DO NOTHING
					`, name, now.UTC()); err != nil {
						return ModuleResult{}, err
					}
					restored++
				}
				return ModuleResult{Counts: map[string]int64{"control_types.builtin_ensured": restored}}, nil
			})
			if err != nil {
				return ModuleResult{}, err
			}
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return res, nil
		},
	}
}

