package monitoring

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type EngineStatsSnapshot struct {
	NowUTC time.Time `json:"now_utc"`

	Tuning Tuning `json:"tuning"`

	InflightChecks       int64 `json:"inflight_checks"`
	MaxConcurrent        int   `json:"max_concurrent"`
	DueCountLastTick     int   `json:"due_count_last_tick"`
	StartedLastTick      int   `json:"started_last_tick"`
	RetryDueLastTick     int   `json:"retry_due_count_last_tick"`
	RetryStartedLastTick int   `json:"retry_started_last_tick"`
	RetryBudgetLastTick  int   `json:"retry_budget_last_tick"`
	SkippedSemaphore     int   `json:"skipped_due_due_to_semaphore_last_tick"`
	SkippedJitter        int   `json:"skipped_due_due_to_jitter_last_tick"`

	WaitTimeSecondsQuantiles   map[string]float64 `json:"check_wait_time_seconds_quantiles"`
	AttemptDurationMsQuantiles map[string]float64 `json:"attempt_duration_ms_quantiles"`
	ErrorClassCounts           map[string]int64   `json:"error_class_counts"`

	AttemptsTotal         int64            `json:"attempts_total"`
	RetryAttemptsTotal    int64            `json:"retry_attempts_total"`
	RetryScheduledTotal   int64            `json:"retry_scheduled_total"`
	RetryAttemptHistogram map[string]int64 `json:"retry_attempt_scheduled_histogram"`
}

type engineObservability struct {
	inflight       atomic.Int64
	attempts       atomic.Int64
	retryAttempts  atomic.Int64
	retryScheduled atomic.Int64

	muTick   sync.Mutex
	lastTick struct {
		due          int
		started      int
		retryDue     int
		retryStarted int
		retryBudget  int
		skipSem      int
		skipJit      int
		at           time.Time
	}

	waitSec ringBuffer
	durMs   ringBuffer

	muErrors sync.Mutex
	errors   map[string]int64

	muRetry   sync.Mutex
	retryHist map[string]int64

	muLog   sync.Mutex
	lastLog time.Time
}

func newEngineObservability() *engineObservability {
	o := &engineObservability{
		errors:    make(map[string]int64),
		retryHist: make(map[string]int64),
	}
	o.waitSec = newRingBuffer(2048)
	o.durMs = newRingBuffer(2048)
	return o
}

func (o *engineObservability) OnAcquireSlot() {
	o.inflight.Add(1)
}

func (o *engineObservability) OnReleaseSlot() {
	o.inflight.Add(-1)
}

func (o *engineObservability) RecordTick(now time.Time, due, started, retryDue, retryStarted, retryBudget, skipSem, skipJit int) {
	o.muTick.Lock()
	o.lastTick.due = due
	o.lastTick.started = started
	o.lastTick.retryDue = retryDue
	o.lastTick.retryStarted = retryStarted
	o.lastTick.retryBudget = retryBudget
	o.lastTick.skipSem = skipSem
	o.lastTick.skipJit = skipJit
	o.lastTick.at = now.UTC()
	o.muTick.Unlock()
}

func (o *engineObservability) RecordWaitTime(wait time.Duration) {
	if wait <= 0 {
		return
	}
	o.waitSec.Add(floatWaitMillis(wait))
}

func (o *engineObservability) RecordAttemptDuration(d time.Duration) {
	if d <= 0 {
		return
	}
	o.durMs.Add(floatMillis(d))
}

func (o *engineObservability) RecordResult(res CheckResult) {
	o.attempts.Add(1)
	class := classifyResult(res)
	o.muErrors.Lock()
	o.errors[class]++
	o.muErrors.Unlock()
}

func (o *engineObservability) RecordRetry(decision RetryDecision) {
	if o == nil {
		return
	}
	if decision.WasRetry {
		o.retryAttempts.Add(1)
	}
	if decision.Scheduled {
		o.retryScheduled.Add(1)
		key := "attempt_" + strconvItoaSafe(decision.RetryAttempt)
		o.muRetry.Lock()
		o.retryHist[key]++
		o.muRetry.Unlock()
	}
}

func (o *engineObservability) Snapshot(now time.Time, tuning Tuning, maxConcurrent int) EngineStatsSnapshot {
	o.muTick.Lock()
	lt := o.lastTick
	o.muTick.Unlock()

	o.muErrors.Lock()
	errs := make(map[string]int64, len(o.errors))
	for k, v := range o.errors {
		errs[k] = v
	}
	o.muErrors.Unlock()

	o.muRetry.Lock()
	retryHist := make(map[string]int64, len(o.retryHist))
	for k, v := range o.retryHist {
		retryHist[k] = v
	}
	o.muRetry.Unlock()

	waitQ := o.waitSec.Quantiles()
	for k, v := range waitQ {
		waitQ[k] = v / 1000.0
	}
	return EngineStatsSnapshot{
		NowUTC:                     now.UTC(),
		Tuning:                     tuning,
		InflightChecks:             o.inflight.Load(),
		MaxConcurrent:              maxConcurrent,
		DueCountLastTick:           lt.due,
		StartedLastTick:            lt.started,
		RetryDueLastTick:           lt.retryDue,
		RetryStartedLastTick:       lt.retryStarted,
		RetryBudgetLastTick:        lt.retryBudget,
		SkippedSemaphore:           lt.skipSem,
		SkippedJitter:              lt.skipJit,
		WaitTimeSecondsQuantiles:   waitQ,
		AttemptDurationMsQuantiles: o.durMs.Quantiles(),
		ErrorClassCounts:           errs,
		AttemptsTotal:              o.attempts.Load(),
		RetryAttemptsTotal:         o.retryAttempts.Load(),
		RetryScheduledTotal:        o.retryScheduled.Load(),
		RetryAttemptHistogram:      retryHist,
	}
}

func (o *engineObservability) MaybeLog(now time.Time, tuning Tuning, maxConcurrent int, logger interface{ Printf(string, ...any) }) {
	if o == nil || logger == nil || tuning.StatsLogInterval <= 0 {
		return
	}
	o.muLog.Lock()
	should := o.lastLog.IsZero() || now.Sub(o.lastLog) >= tuning.StatsLogInterval
	if should {
		o.lastLog = now
	}
	o.muLog.Unlock()
	if !should {
		return
	}
	snap := o.Snapshot(now, tuning, maxConcurrent)
	retryShare := 0.0
	if snap.AttemptsTotal > 0 {
		retryShare = float64(snap.RetryAttemptsTotal) / float64(snap.AttemptsTotal)
	}
	logger.Printf(
		"MONITORING_STATS inflight=%d max=%d due_last=%d started_last=%d retry_due_last=%d retry_started_last=%d retry_budget_last=%d skipped_sem_last=%d skipped_jitter_last=%d wait_p95=%.3fs dur_p95=%.0fms retry_scheduled=%d retry_share=%.2f",
		snap.InflightChecks,
		snap.MaxConcurrent,
		snap.DueCountLastTick,
		snap.StartedLastTick,
		snap.RetryDueLastTick,
		snap.RetryStartedLastTick,
		snap.RetryBudgetLastTick,
		snap.SkippedSemaphore,
		snap.SkippedJitter,
		snap.WaitTimeSecondsQuantiles["p95"],
		snap.AttemptDurationMsQuantiles["p95"],
		snap.RetryScheduledTotal,
		retryShare,
	)
}

func strconvItoaSafe(v int) string {
	if v <= 0 {
		return "0"
	}
	if v > 1000 {
		return "1000+"
	}
	return strconv.Itoa(v)
}

func classifyResult(res CheckResult) string {
	if res.OK {
		return "ok"
	}
	err := strings.TrimSpace(res.Error)
	if err == "" {
		return "unknown"
	}
	if strings.HasPrefix(err, "status_") {
		return "http_status"
	}
	if strings.Contains(err, "timeout") || strings.HasSuffix(err, ".timeout") || strings.Contains(err, "DeadlineExceeded") {
		return "timeout"
	}
	if strings.Contains(err, "dns") || strings.Contains(err, "no such host") {
		return "dns"
	}
	if strings.Contains(err, "tls") || strings.Contains(err, "x509") {
		return "tls"
	}
	if strings.Contains(err, "keyword") {
		return "keyword"
	}
	if strings.Contains(err, "json") {
		return "json"
	}
	return "request_failed"
}

func floatWaitMillis(d time.Duration) int64 {
	return int64(d.Milliseconds())
}

func floatMillis(d time.Duration) int64 {
	return int64(d.Milliseconds())
}

type ringBuffer struct {
	mu     sync.Mutex
	values []int64
	next   int
	filled bool
}

func newRingBuffer(size int) ringBuffer {
	if size <= 0 {
		size = 1
	}
	return ringBuffer{values: make([]int64, size)}
}

func (r *ringBuffer) Add(v int64) {
	r.mu.Lock()
	r.values[r.next] = v
	r.next++
	if r.next >= len(r.values) {
		r.next = 0
		r.filled = true
	}
	r.mu.Unlock()
}

func (r *ringBuffer) Quantiles() map[string]float64 {
	r.mu.Lock()
	var n int
	if r.filled {
		n = len(r.values)
	} else {
		n = r.next
	}
	cpy := make([]int64, 0, n)
	cpy = append(cpy, r.values[:n]...)
	r.mu.Unlock()

	if len(cpy) == 0 {
		return map[string]float64{"p50": 0, "p95": 0, "p99": 0}
	}
	sort.Slice(cpy, func(i, j int) bool { return cpy[i] < cpy[j] })
	return map[string]float64{
		"p50": quantile(cpy, 0.50),
		"p95": quantile(cpy, 0.95),
		"p99": quantile(cpy, 0.99),
	}
}

func quantile(sorted []int64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return float64(sorted[0])
	}
	if q >= 1 {
		return float64(sorted[len(sorted)-1])
	}
	pos := int(float64(len(sorted)-1) * q)
	if pos < 0 {
		pos = 0
	}
	if pos >= len(sorted) {
		pos = len(sorted) - 1
	}
	return float64(sorted[pos])
}
