package appjobs

import (
	"context"
	"database/sql"
	"strings"

	"berkut-scc/core/appcompat"
)

func moduleMonitoring(spec appcompat.ModuleSpec) Module {
	return moduleSpec{
		id:               spec.ModuleID,
		hasFullReset:     spec.HasFullReset,
		expectedSchema:   spec.ExpectedSchemaVersion,
		expectedBehavior: spec.ExpectedBehaviorVersion,
		full: func(ctx context.Context, deps ModuleDeps) (ModuleResult, error) {
			res, err := withTx(ctx, deps.DB, func(tx *sql.Tx) (ModuleResult, error) {
				tables := []string{
					"monitor_notification_deliveries",
					"monitor_notifications",
					"monitor_notification_state",
					"notification_channels",
					"monitor_maintenance",
					"monitor_tls",
					"monitor_events",
					"monitor_metrics",
					"monitor_assets",
					"monitor_state",
					"monitor_sla_period_results",
					"monitor_sla_policies",
					"monitors",
					"monitoring_settings",
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
				// Cleanup invalid retry state after version bumps.
				if exists, _ := tableExists(ctx, tx, "monitor_state"); exists {
					r1, err := tx.ExecContext(ctx, `
						UPDATE monitor_state
						SET retry_attempt=0, retry_at=NULL, last_error_kind=''
						WHERE retry_attempt<>0 OR retry_at IS NOT NULL OR (last_error_kind IS NOT NULL AND TRIM(last_error_kind)<>'')
					`)
					if err == nil && r1 != nil {
						n, _ := r1.RowsAffected()
						counts["monitor_state.retry_reset"] = n
					}
				}
				// Remove any orphan state rows (should not happen due to FK).
				if exists, _ := tableExists(ctx, tx, "monitor_state"); exists {
					n, err := deleteWhere(ctx, tx, "monitor_state", "monitor_id NOT IN (SELECT id FROM monitors)")
					if err == nil {
						counts["monitor_state.orphans"] = n
					}
				}
				// Normalize empty last_result_status.
				if exists, _ := tableExists(ctx, tx, "monitor_state"); exists {
					r2, err := tx.ExecContext(ctx, `
						UPDATE monitor_state
						SET last_result_status=''
						WHERE last_result_status IS NULL
					`)
					if err == nil && r2 != nil {
						n, _ := r2.RowsAffected()
						if n > 0 {
							counts["monitor_state.last_result_status_normalized"] = n
						}
					}
				}
				// Ensure monitoring_settings exists or app will fallback to defaults; no DB write needed.
				if len(counts) == 0 {
					counts["noop"] = 1
				}
				// Strip accidental whitespace in monitors.url fields.
				if exists, _ := tableExists(ctx, tx, "monitors"); exists {
					r3, err := tx.ExecContext(ctx, `
						UPDATE monitors
						SET url=TRIM(url)
						WHERE url IS NOT NULL AND url<>TRIM(url)
					`)
					if err == nil && r3 != nil {
						n, _ := r3.RowsAffected()
						if n > 0 {
							counts["monitors.url_trimmed"] = n
						}
					}
				}
				// Clear any maintenance windows with invalid end before start.
				if exists, _ := tableExists(ctx, tx, "monitor_maintenance"); exists {
					n, err := deleteWhere(ctx, tx, "monitor_maintenance", "ends_at < starts_at")
					if err == nil && n > 0 {
						counts["monitor_maintenance.invalid_deleted"] = n
					}
				}
				// Ensure status is known in monitor_notifications.
				if exists, _ := tableExists(ctx, tx, "monitor_notification_deliveries"); exists {
					r4, err := tx.ExecContext(ctx, `
						UPDATE monitor_notification_deliveries
						SET status=LOWER(TRIM(status))
						WHERE status IS NOT NULL AND status<>LOWER(TRIM(status))
					`)
					if err == nil && r4 != nil {
						n, _ := r4.RowsAffected()
						if n > 0 {
							counts["monitor_notification_deliveries.status_normalized"] = n
						}
					}
				}
				// Remove empty telegram channel names.
				if exists, _ := tableExists(ctx, tx, "notification_channels"); exists {
					r5, err := tx.ExecContext(ctx, `
						UPDATE notification_channels
						SET name='channel'
						WHERE name IS NULL OR TRIM(name)=''
					`)
					if err == nil && r5 != nil {
						n, _ := r5.RowsAffected()
						if n > 0 {
							counts["notification_channels.name_defaulted"] = n
						}
					}
				}
				// Normalize monitor types.
				if exists, _ := tableExists(ctx, tx, "monitors"); exists {
					r6, err := tx.ExecContext(ctx, `
						UPDATE monitors
						SET type=LOWER(TRIM(type))
						WHERE type IS NOT NULL AND type<>LOWER(TRIM(type))
					`)
					if err == nil && r6 != nil {
						n, _ := r6.RowsAffected()
						if n > 0 {
							counts["monitors.type_normalized"] = n
						}
					}
					// Ensure type not empty.
					r7, err := tx.ExecContext(ctx, `
						UPDATE monitors
						SET type='http'
						WHERE type IS NULL OR TRIM(type)=''
					`)
					if err == nil && r7 != nil {
						n, _ := r7.RowsAffected()
						if n > 0 {
							counts["monitors.type_defaulted"] = n
						}
					}
				}
				// Ensure monitor_state.last_error_kind is not whitespace.
				if exists, _ := tableExists(ctx, tx, "monitor_state"); exists {
					r8, err := tx.ExecContext(ctx, `
						UPDATE monitor_state
						SET last_error_kind=''
						WHERE last_error_kind IS NOT NULL AND TRIM(last_error_kind)=''
					`)
					if err == nil && r8 != nil {
						n, _ := r8.RowsAffected()
						if n > 0 {
							counts["monitor_state.last_error_kind_normalized"] = n
						}
					}
				}
				// Keep counts keys stable.
				for k := range counts {
					counts[strings.TrimSpace(k)] = counts[k]
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

