package api

import (
	"berkut-scc/core/appjobs"
	"berkut-scc/core/backups"
	"berkut-scc/core/monitoring"
	"berkut-scc/tasks"
	"github.com/prometheus/client_golang/prometheus"
)

type workersMetricsCollector struct {
	tasks      *tasks.RecurringScheduler
	backups    *backups.Scheduler
	appJobs    *appjobs.Worker
	monitoring *monitoring.Engine

	ticksTotalDesc      *prometheus.Desc
	tickErrorsTotalDesc *prometheus.Desc
	lastTickDesc        *prometheus.Desc
}

func newWorkersMetricsCollector(tasksScheduler *tasks.RecurringScheduler, backupsScheduler *backups.Scheduler, appJobsWorker *appjobs.Worker, monitoringEngine *monitoring.Engine) prometheus.Collector {
	return &workersMetricsCollector{
		tasks:      tasksScheduler,
		backups:    backupsScheduler,
		appJobs:    appJobsWorker,
		monitoring: monitoringEngine,
		ticksTotalDesc: prometheus.NewDesc(
			"berkut_worker_ticks_total",
			"Total number of scheduler/worker ticks.",
			[]string{"worker"},
			nil,
		),
		tickErrorsTotalDesc: prometheus.NewDesc(
			"berkut_worker_tick_errors_total",
			"Total number of scheduler/worker tick errors.",
			[]string{"worker"},
			nil,
		),
		lastTickDesc: prometheus.NewDesc(
			"berkut_worker_last_tick_timestamp",
			"Unix timestamp of the last scheduler/worker tick.",
			[]string{"worker"},
			nil,
		),
	}
}

func (c *workersMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.ticksTotalDesc
	ch <- c.tickErrorsTotalDesc
	ch <- c.lastTickDesc
}

func (c *workersMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if c == nil {
		return
	}
	if c.tasks != nil {
		s := c.tasks.StatsSnapshot()
		ch <- prometheus.MustNewConstMetric(c.ticksTotalDesc, prometheus.CounterValue, float64(s.TicksTotal), "tasks_recurring")
		ch <- prometheus.MustNewConstMetric(c.tickErrorsTotalDesc, prometheus.CounterValue, float64(s.TickErrorsTotal), "tasks_recurring")
		if s.LastTickAtUTC != nil {
			ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(s.LastTickAtUTC.UTC().Unix()), "tasks_recurring")
		}
	}
	if c.backups != nil {
		s := c.backups.StatsSnapshot()
		ch <- prometheus.MustNewConstMetric(c.ticksTotalDesc, prometheus.CounterValue, float64(s.TicksTotal), "backups_scheduler")
		ch <- prometheus.MustNewConstMetric(c.tickErrorsTotalDesc, prometheus.CounterValue, float64(s.TickErrorsTotal), "backups_scheduler")
		if s.LastTickAtUTC != nil {
			ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(s.LastTickAtUTC.UTC().Unix()), "backups_scheduler")
		}
	}
	if c.appJobs != nil {
		s := c.appJobs.StatsSnapshot()
		ch <- prometheus.MustNewConstMetric(c.ticksTotalDesc, prometheus.CounterValue, float64(s.TicksTotal), "app_jobs_worker")
		ch <- prometheus.MustNewConstMetric(c.tickErrorsTotalDesc, prometheus.CounterValue, float64(s.TickErrorsTotal), "app_jobs_worker")
		if s.LastTickAtUTC != nil {
			ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(s.LastTickAtUTC.UTC().Unix()), "app_jobs_worker")
		}
	}

	// Monitoring engine has its own stats model; export a basic tick timestamp based on the snapshot time.
	if c.monitoring != nil {
		snap := c.monitoring.StatsSnapshot()
		ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.NowUTC.UTC().Unix()), "monitoring_engine")
	}
}
