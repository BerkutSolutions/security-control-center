package api

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type backupsMetricsCollector struct {
	db *sql.DB

	artifactsCountDesc *prometheus.Desc
	runsCountDesc      *prometheus.Desc
	restoreCountDesc   *prometheus.Desc

	planEnabledDesc       *prometheus.Desc
	planLastAutoRunDesc   *prometheus.Desc
	planLastAutoAgeDesc   *prometheus.Desc
	planLastAutoErrorDesc *prometheus.Desc
}

func newBackupsMetricsCollector(db *sql.DB) prometheus.Collector {
	return &backupsMetricsCollector{
		db: db,
		artifactsCountDesc: prometheus.NewDesc(
			"berkut_backups_artifacts_count",
			"Number of backup artifacts by status.",
			[]string{"status"},
			nil,
		),
		runsCountDesc: prometheus.NewDesc(
			"berkut_backups_runs_count",
			"Number of backup runs by status.",
			[]string{"status"},
			nil,
		),
		restoreCountDesc: prometheus.NewDesc(
			"berkut_backups_restore_runs_count",
			"Number of backup restore runs by status and mode.",
			[]string{"status", "dry_run"},
			nil,
		),
		planEnabledDesc: prometheus.NewDesc(
			"berkut_backup_plan_enabled",
			"Whether backup plan is enabled (1) or disabled (0).",
			nil,
			nil,
		),
		planLastAutoRunDesc: prometheus.NewDesc(
			"berkut_backup_plan_last_auto_run_timestamp",
			"Unix timestamp of the last automatic backup run.",
			nil,
			nil,
		),
		planLastAutoAgeDesc: prometheus.NewDesc(
			"berkut_backup_plan_last_auto_run_age_seconds",
			"Age of the last automatic backup run in seconds.",
			nil,
			nil,
		),
		planLastAutoErrorDesc: prometheus.NewDesc(
			"berkut_backup_plan_query_error",
			"Whether backup plan metrics query failed (1) or succeeded (0).",
			nil,
			nil,
		),
	}
}

func (c *backupsMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.artifactsCountDesc
	ch <- c.runsCountDesc
	ch <- c.restoreCountDesc
	ch <- c.planEnabledDesc
	ch <- c.planLastAutoRunDesc
	ch <- c.planLastAutoAgeDesc
	ch <- c.planLastAutoErrorDesc
}

func (c *backupsMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if c == nil || c.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()

	collectCountByStatus(ctx, c.db, ch, c.artifactsCountDesc, `SELECT status, COUNT(*) FROM backups_artifacts GROUP BY status`)
	collectCountByStatus(ctx, c.db, ch, c.runsCountDesc, `SELECT status, COUNT(*) FROM backups_runs GROUP BY status`)

	if rows, err := c.db.QueryContext(ctx, `SELECT status, dry_run, COUNT(*) FROM backups_restore_runs GROUP BY status, dry_run`); err == nil {
		for rows.Next() {
			var status string
			var dryRun bool
			var n float64
			if scanErr := rows.Scan(&status, &dryRun, &n); scanErr == nil {
				dry := "false"
				if dryRun {
					dry = "true"
				}
				ch <- prometheus.MustNewConstMetric(c.restoreCountDesc, prometheus.GaugeValue, n, status, dry)
			}
		}
		_ = rows.Close()
	}

	enabled, lastAuto, ok := queryBackupPlan(ctx, c.db)
	if !ok {
		ch <- prometheus.MustNewConstMetric(c.planLastAutoErrorDesc, prometheus.GaugeValue, 1)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.planLastAutoErrorDesc, prometheus.GaugeValue, 0)
	if enabled {
		ch <- prometheus.MustNewConstMetric(c.planEnabledDesc, prometheus.GaugeValue, 1)
	} else {
		ch <- prometheus.MustNewConstMetric(c.planEnabledDesc, prometheus.GaugeValue, 0)
	}
	if lastAuto != nil && !lastAuto.IsZero() {
		ts := float64(lastAuto.UTC().Unix())
		ch <- prometheus.MustNewConstMetric(c.planLastAutoRunDesc, prometheus.GaugeValue, ts)
		age := time.Since(lastAuto.UTC()).Seconds()
		if age >= 0 {
			ch <- prometheus.MustNewConstMetric(c.planLastAutoAgeDesc, prometheus.GaugeValue, age)
		}
	}
}

func collectCountByStatus(ctx context.Context, db *sql.DB, ch chan<- prometheus.Metric, desc *prometheus.Desc, query string) {
	if db == nil {
		return
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n float64
		if scanErr := rows.Scan(&status, &n); scanErr == nil {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, n, status)
		}
	}
}

func queryBackupPlan(ctx context.Context, db *sql.DB) (enabled bool, lastAuto *time.Time, ok bool) {
	if db == nil {
		return false, nil, false
	}
	row := db.QueryRowContext(ctx, `SELECT enabled, last_auto_run_at FROM backup_plans ORDER BY id LIMIT 1`)
	var enabledInt int
	var last sql.NullTime
	if err := row.Scan(&enabledInt, &last); err != nil {
		// If plan is not configured yet, treat as ok but missing.
		return false, nil, true
	}
	enabled = enabledInt != 0
	if last.Valid {
		t := last.Time.UTC()
		lastAuto = &t
	}
	return enabled, lastAuto, true
}
