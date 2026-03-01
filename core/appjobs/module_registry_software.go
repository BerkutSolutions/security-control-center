package appjobs

import (
	"context"
	"database/sql"

	"berkut-scc/core/appcompat"
)

func moduleRegistrySoftware(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"asset_software",
					"software_versions",
					"software_products",
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
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				counts := map[string]int64{}
				if exists, _ := tableExists(ctx, tx, "software_versions"); exists {
					n, err := deleteWhere(ctx, tx, "software_versions", "product_id NOT IN (SELECT id FROM software_products)")
					if err == nil && n > 0 {
						counts["software_versions.orphans_deleted"] = n
					}
				}
				if exists, _ := tableExists(ctx, tx, "asset_software"); exists {
					n, err := deleteWhere(ctx, tx, "asset_software", "product_id NOT IN (SELECT id FROM software_products)")
					if err == nil && n > 0 {
						counts["asset_software.orphans_product_deleted"] = n
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

