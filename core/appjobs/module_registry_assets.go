package appjobs

import (
	"context"
	"database/sql"

	"berkut-scc/core/appcompat"
)

func moduleRegistryAssets(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"asset_software",
					"monitor_assets",
					"assets",
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
				if exists, _ := tableExists(ctx, tx, "asset_software"); exists {
					n, err := deleteWhere(ctx, tx, "asset_software", "asset_id NOT IN (SELECT id FROM assets)")
					if err == nil && n > 0 {
						counts["asset_software.orphans_asset_deleted"] = n
					}
					n2, err := deleteWhere(ctx, tx, "asset_software", "product_id NOT IN (SELECT id FROM software_products)")
					if err == nil && n2 > 0 {
						counts["asset_software.orphans_product_deleted"] = n2
					}
				}
				if exists, _ := tableExists(ctx, tx, "monitor_assets"); exists {
					n, err := deleteWhere(ctx, tx, "monitor_assets", "asset_id NOT IN (SELECT id FROM assets)")
					if err == nil && n > 0 {
						counts["monitor_assets.orphans_asset_deleted"] = n
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

