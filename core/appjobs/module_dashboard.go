package appjobs

import (
	"context"
	"database/sql"

	"berkut-scc/core/appcompat"
)

func moduleDashboard(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts, err := deleteTablesInOrder(ctx, tx, []string{"dashboard_layouts"})
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
			// Nothing to backfill yet; mark compatible.
			_ = upsertModuleStateCompatible(ctx, deps, spec.ModuleID, spec.ExpectedSchemaVersion, spec.ExpectedBehaviorVersion, "")
			return ModuleResult{Counts: map[string]int64{"noop": 1}}, nil
		},
	}
}
