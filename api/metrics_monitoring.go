package api

import (
	"berkut-scc/core/monitoring"
	"github.com/prometheus/client_golang/prometheus"
)

type monitoringMetricsCollector struct {
	engine *monitoring.Engine

	inflightDesc            *prometheus.Desc
	maxConcurrentDesc       *prometheus.Desc
	lastTickDesc            *prometheus.Desc
	quantileWaitDesc        *prometheus.Desc
	quantileAttemptMsDesc   *prometheus.Desc
	errorClassDesc          *prometheus.Desc
	attemptsTotalDesc       *prometheus.Desc
	retryAttemptsTotalDesc  *prometheus.Desc
	retryScheduledTotalDesc *prometheus.Desc
	retryAttemptHistDesc    *prometheus.Desc
}

func newMonitoringMetricsCollector(engine *monitoring.Engine) prometheus.Collector {
	return &monitoringMetricsCollector{
		engine: engine,
		inflightDesc: prometheus.NewDesc(
			"berkut_monitoring_inflight_checks",
			"Current number of in-flight monitoring checks.",
			nil,
			nil,
		),
		maxConcurrentDesc: prometheus.NewDesc(
			"berkut_monitoring_max_concurrent_checks",
			"Current max concurrent checks limit used by monitoring engine.",
			nil,
			nil,
		),
		lastTickDesc: prometheus.NewDesc(
			"berkut_monitoring_last_tick",
			"Monitoring engine last tick counters (from last 1-second tick).",
			[]string{"kind"},
			nil,
		),
		quantileWaitDesc: prometheus.NewDesc(
			"berkut_monitoring_check_wait_time_seconds",
			"Monitoring check start wait time quantiles in seconds.",
			[]string{"quantile"},
			nil,
		),
		quantileAttemptMsDesc: prometheus.NewDesc(
			"berkut_monitoring_attempt_duration_ms",
			"Monitoring attempt duration quantiles in milliseconds.",
			[]string{"quantile"},
			nil,
		),
		errorClassDesc: prometheus.NewDesc(
			"berkut_monitoring_error_class_total",
			"Total number of monitoring check results by error class (including ok).",
			[]string{"class"},
			nil,
		),
		attemptsTotalDesc: prometheus.NewDesc(
			"berkut_monitoring_attempts_total",
			"Total number of monitoring attempts.",
			nil,
			nil,
		),
		retryAttemptsTotalDesc: prometheus.NewDesc(
			"berkut_monitoring_retry_attempts_total",
			"Total number of monitoring retry attempts.",
			nil,
			nil,
		),
		retryScheduledTotalDesc: prometheus.NewDesc(
			"berkut_monitoring_retry_scheduled_total",
			"Total number of scheduled retries.",
			nil,
			nil,
		),
		retryAttemptHistDesc: prometheus.NewDesc(
			"berkut_monitoring_retry_attempt_scheduled_total",
			"Total number of scheduled retries by retry attempt index.",
			[]string{"attempt"},
			nil,
		),
	}
}

func (c *monitoringMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.inflightDesc
	ch <- c.maxConcurrentDesc
	ch <- c.lastTickDesc
	ch <- c.quantileWaitDesc
	ch <- c.quantileAttemptMsDesc
	ch <- c.errorClassDesc
	ch <- c.attemptsTotalDesc
	ch <- c.retryAttemptsTotalDesc
	ch <- c.retryScheduledTotalDesc
	ch <- c.retryAttemptHistDesc
}

func (c *monitoringMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	if c == nil || c.engine == nil {
		return
	}
	snap := c.engine.StatsSnapshot()

	ch <- prometheus.MustNewConstMetric(c.inflightDesc, prometheus.GaugeValue, float64(snap.InflightChecks))
	ch <- prometheus.MustNewConstMetric(c.maxConcurrentDesc, prometheus.GaugeValue, float64(snap.MaxConcurrent))

	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.DueCountLastTick), "due")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.StartedLastTick), "started")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.RetryDueLastTick), "retry_due")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.RetryStartedLastTick), "retry_started")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.RetryBudgetLastTick), "retry_budget")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.SkippedSemaphore), "skipped_semaphore")
	ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue, float64(snap.SkippedJitter), "skipped_jitter")

	for q, v := range snap.WaitTimeSecondsQuantiles {
		ch <- prometheus.MustNewConstMetric(c.quantileWaitDesc, prometheus.GaugeValue, v, q)
	}
	for q, v := range snap.AttemptDurationMsQuantiles {
		ch <- prometheus.MustNewConstMetric(c.quantileAttemptMsDesc, prometheus.GaugeValue, v, q)
	}
	for class, n := range snap.ErrorClassCounts {
		ch <- prometheus.MustNewConstMetric(c.errorClassDesc, prometheus.CounterValue, float64(n), class)
	}

	ch <- prometheus.MustNewConstMetric(c.attemptsTotalDesc, prometheus.CounterValue, float64(snap.AttemptsTotal))
	ch <- prometheus.MustNewConstMetric(c.retryAttemptsTotalDesc, prometheus.CounterValue, float64(snap.RetryAttemptsTotal))
	ch <- prometheus.MustNewConstMetric(c.retryScheduledTotalDesc, prometheus.CounterValue, float64(snap.RetryScheduledTotal))

	for attempt, n := range snap.RetryAttemptHistogram {
		ch <- prometheus.MustNewConstMetric(c.retryAttemptHistDesc, prometheus.CounterValue, float64(n), attempt)
	}
}
