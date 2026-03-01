package appjobs

import (
	"context"
	"database/sql"

	"berkut-scc/core/appcompat"
)

func moduleSettings(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"app_https_settings",
					"app_runtime_settings",
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
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return res, nil
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			// Settings are defaulted when missing; mark compatible.
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return ModuleResult{Counts: map[string]int64{"noop": 1}}, nil
		},
	}
}

