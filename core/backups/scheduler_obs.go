package backups

import (
	"sync/atomic"
	"time"
)

type SchedulerStats struct {
	TicksTotal      uint64     `json:"ticks_total"`
	TickErrorsTotal uint64     `json:"tick_errors_total"`
	LastTickAtUTC   *time.Time `json:"last_tick_at_utc,omitempty"`
}

type schedulerObs struct {
	ticks      atomic.Uint64
	tickErrors atomic.Uint64
	lastTickNs atomic.Int64
}

func (o *schedulerObs) recordTick(now time.Time, err error) {
	if o == nil {
		return
	}
	o.ticks.Add(1)
	if err != nil {
		o.tickErrors.Add(1)
	}
	o.lastTickNs.Store(now.UTC().UnixNano())
}

func (s *Scheduler) StatsSnapshot() SchedulerStats {
	if s == nil {
		return SchedulerStats{}
	}
	ns := s.obs.lastTickNs.Load()
	var last *time.Time
	if ns > 0 {
		t := time.Unix(0, ns).UTC()
		last = &t
	}
	return SchedulerStats{
		TicksTotal:      s.obs.ticks.Load(),
		TickErrorsTotal: s.obs.tickErrors.Load(),
		LastTickAtUTC:   last,
	}
}
