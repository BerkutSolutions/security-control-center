package appjobs

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"berkut-scc/core/appcompat"
)

func moduleAccounts(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			return ModuleResult{}, errors.New("accounts full reset is not supported")
		},
		partial: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				now := deps.NowUTC
				if now.IsZero() {
					now = time.Now().UTC()
				}
				counts := map[string]int64{}
				if exists, _ := tableExists(ctx, tx, "sessions"); exists {
					r, err := tx.ExecContext(ctx, `
						DELETE FROM sessions
						WHERE revoked=1 OR expires_at < ?
					`, now.UTC())
					if err == nil && r != nil {
						n, _ := r.RowsAffected()
						counts["sessions.cleaned"] = n
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

