package api

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type appJobsMetricsCollector struct {
	db *sql.DB

	jobCountDesc       *prometheus.Desc
	oldestQueuedAgeSec *prometheus.Desc
}

func newAppJobsMetricsCollector(db *sql.DB) prometheus.Collector {
	return &appJobsMetricsCollector{
		db: db,
		jobCountDesc: prometheus.NewDesc(
			"berkut_app_jobs_count",
			"Number of app jobs by status.",
			[]string{"status"},
			nil,
		),
		oldestQueuedAgeSec: prometheus.NewDesc(
			"berkut_app_jobs_oldest_queued_age_seconds",
			"Age of the oldest queued app job in seconds.",
			nil,
			nil,
		),
	}
}

func (c *appJobsMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.jobCountDesc
	ch <- c.oldestQueuedAgeSec
}

func (c *appJobsMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if c == nil || c.db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	counts := map[string]float64{}
	rows, err := c.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM app_jobs GROUP BY status`)
	if err == nil {
		for rows.Next() {
			var status string
			var n float64
			if scanErr := rows.Scan(&status, &n); scanErr == nil {
				counts[status] = n
			}
		}
		_ = rows.Close()
	}
	for status, n := range counts {
		ch <- prometheus.MustNewConstMetric(c.jobCountDesc, prometheus.GaugeValue, n, status)
	}

	var oldestQueuedAgeSec sql.NullFloat64
	row := c.db.QueryRowContext(ctx, `
		SELECT EXTRACT(EPOCH FROM (NOW() - MIN(created_at)))
		FROM app_jobs
		WHERE status='queued'
	`)
	if err := row.Scan(&oldestQueuedAgeSec); err == nil && oldestQueuedAgeSec.Valid && oldestQueuedAgeSec.Float64 >= 0 {
		ch <- prometheus.MustNewConstMetric(c.oldestQueuedAgeSec, prometheus.GaugeValue, oldestQueuedAgeSec.Float64)
	}
}
